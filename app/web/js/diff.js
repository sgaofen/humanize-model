// 词级差异:英文按词,中文按字,标点单独一个。只比较非空白 token,渲染时把空白原样放回。
//
// 1. 去掉公共前后缀,剩下的做 LCS(动态规划,O(n·m),上限 2500 万格);
// 2. 语义清理(参考 diff-match-patch 的 cleanupSemantic,但更保守):夹在两段改动之间、
//    长度不到两边改动一半的"相同"片段,改记为改动 —— 否则重写得很深的段落会被
//    "the"、"，"、"的" 这种碰巧对上的小词切成满屏碎片。

const TOKEN_RE = /\p{Nd}+(?:[.,:/]\p{Nd}+)+|[\u3400-\u4DBF\u4E00-\u9FFF\uF900-\uFAFF\u3040-\u30FF\uAC00-\uD7AF]|[\p{L}\p{N}\p{M}_]+(?:['\u2019][\p{L}\p{N}]+)*|\s+|[^\s]/gu;
const CJK_ONE = /^[\u3400-\u4DBF\u4E00-\u9FFF\uF900-\uFAFF\u3040-\u30FF\uAC00-\uD7AF]$/;
const PUNCT = /^[\p{P}\p{S}]+$/u;
const MAX_CELLS = 25e6;

function norm(t) {
  return t.normalize('NFKC').replace(/[’‘]/g, "'").replace(/[“”]/g, '"');
}

/** 切成 token;ws=true 的是空白。content 下标 ci 只给非空白 token。 */
export function tokenize(s) {
  const out = [];
  let ci = 0;
  for (const m of s.matchAll(TOKEN_RE)) {
    const t = m[0];
    if (/^\s+$/.test(t)) out.push({ t, ws: true, ci: -1 });
    else {
      // 权重:一个汉字的信息量大约相当于一个短英文词
      const w = CJK_ONE.test(t) ? 2 : [...t].length;
      out.push({ t, ws: false, ci: ci++, n: norm(t), w });
    }
  }
  return out;
}

/**
 * 返回 { a: Uint8Array, b: Uint8Array, ratio }:
 *   a[i]=1 表示草稿第 i 个内容 token 被删/改;b[j]=1 表示改写第 j 个内容 token 是新写的。
 *   ratio = 新写内容(按权重)占改写全文的比例。太大算不了时返回 null。
 */
export function diffTokens(aTok, bTok) {
  const A = aTok.filter((x) => !x.ws), B = bTok.filter((x) => !x.ws);
  // 字符串 → 整数,比较快
  const ids = new Map();
  const id = (s) => { let v = ids.get(s); if (v === undefined) { v = ids.size; ids.set(s, v); } return v; };
  const a = Int32Array.from(A, (x) => id(x.n)), b = Int32Array.from(B, (x) => id(x.n));

  let pre = 0;
  while (pre < a.length && pre < b.length && a[pre] === b[pre]) pre++;
  let suf = 0;
  while (suf < a.length - pre && suf < b.length - pre && a[a.length - 1 - suf] === b[b.length - 1 - suf]) suf++;
  const n = a.length - pre - suf, m = b.length - pre - suf;
  if ((n + 1) * (m + 1) > MAX_CELLS) return null;

  // dp[i][j] = a[pre+i..] 与 b[pre+j..] 的 LCS 长度(后缀形式,便于正向回溯)
  const W = m + 1;
  const dp = (n < 65535 && m < 65535) ? new Uint16Array((n + 1) * W) : new Uint32Array((n + 1) * W);
  for (let i = n - 1; i >= 0; i--) {
    const ai = a[pre + i], row = i * W, next = row + W;
    for (let j = m - 1; j >= 0; j--) {
      dp[row + j] = ai === b[pre + j] ? dp[next + j + 1] + 1 : Math.max(dp[next + j], dp[row + j + 1]);
    }
  }

  // ops: 0 相同(ai, bj) / -1 删(ai) / 1 增(bj)
  const ops = [];
  for (let k = 0; k < pre; k++) ops.push([0, k, k]);
  let i = 0, j = 0;
  while (i < n && j < m) {
    if (a[pre + i] === b[pre + j]) { ops.push([0, pre + i, pre + j]); i++; j++; }
    else if (dp[(i + 1) * W + j] >= dp[i * W + j + 1]) { ops.push([-1, pre + i, -1]); i++; }
    else { ops.push([1, -1, pre + j]); j++; }
  }
  for (; i < n; i++) ops.push([-1, pre + i, -1]);
  for (; j < m; j++) ops.push([1, -1, pre + j]);
  for (let k = 0; k < suf; k++) ops.push([0, a.length - suf + k, b.length - suf + k]);

  const clean = cleanup(ops, A, B);
  const am = new Uint8Array(A.length), bm = new Uint8Array(B.length);
  let insW = 0, allW = 0;
  for (const [op, x, y] of clean) {
    if (op === -1) am[x] = 1;
    else if (op === 1) bm[y] = 1;
  }
  quietPunct(am, A);
  quietPunct(bm, B);
  B.forEach((t, k) => { allW += t.w; if (bm[k]) insW += t.w; });
  return { a: am, b: bm, ratio: allW ? insW / allW : 0 };
}

// 只换了一两个标点(，→、 之类)不算改写,不标,免得满屏小黄点。
function quietPunct(marks, toks) {
  for (let i = 0; i < marks.length;) {
    if (!marks[i]) { i++; continue; }
    let j = i;
    while (j < marks.length && marks[j]) j++;
    if (j - i <= 2 && toks.slice(i, j).every((t) => PUNCT.test(t.t))) marks.fill(0, i, j);
    i = j;
  }
}

function cleanup(ops, A, B) {
  for (let pass = 0; pass < 6; pass++) {
    // 分组:相同段 / 改动段(删和增混在一起)
    const groups = [];
    for (const op of ops) {
      const kind = op[0] === 0 ? 0 : 1;
      let g = groups[groups.length - 1];
      if (!g || g.kind !== kind) { g = { kind, ops: [], len: 0, del: 0, ins: 0 }; groups.push(g); }
      g.ops.push(op);
      if (op[0] === 0) g.len += A[op[1]].w;
      else if (op[0] === -1) g.del += A[op[1]].w;
      else g.ins += B[op[2]].w;
    }
    let changed = false;
    for (let k = 1; k < groups.length - 1; k++) {
      const g = groups[k];
      if (g.kind !== 0) continue;
      const p = groups[k - 1], q = groups[k + 1];
      // 相同片段不到两侧改动的一半长才并进去;太宽松会把真没改的词也标成改动
      if (g.len * 2 <= Math.max(p.del, p.ins) && g.len * 2 <= Math.max(q.del, q.ins)) {
        g.kind = 1;
        g.ops = g.ops.flatMap(([, x, y]) => [[-1, x, -1], [1, -1, y]]);
        changed = true;
      }
    }
    if (!changed) return ops;
    ops = groups.flatMap((g) => g.ops);
  }
  return ops;
}

/**
 * 把 token 渲染进容器;marks 命中的内容 token 包进 <span class=cls>。
 * 相邻两个被标记的词之间的空格一起标(看起来像一笔划过去),换行不标。
 */
export function renderMarked(el, tokens, marks, cls) {
  const frag = document.createDocumentFragment();
  let buf = '', span = null;
  const flushText = () => { if (buf) { frag.append(buf); buf = ''; } };
  const isMarked = (k) => {
    const tk = tokens[k];
    if (!tk.ws) return !!marks[tk.ci];
    if (tk.t.includes('\n')) return false;
    let p = k - 1, q = k + 1;
    while (p >= 0 && tokens[p].ws) p--;
    while (q < tokens.length && tokens[q].ws) q++;
    return p >= 0 && q < tokens.length && !!marks[tokens[p].ci] && !!marks[tokens[q].ci];
  };
  for (let k = 0; k < tokens.length; k++) {
    if (isMarked(k)) {
      if (!span) { flushText(); span = document.createElement('span'); span.className = cls; frag.append(span); }
      span.append(tokens[k].t);
    } else {
      span = null;
      buf += tokens[k].t;
    }
  }
  flushText();
  el.replaceChildren(frag);
}

/** 在已渲染的草稿里给指定数字加虚线框(数字核对用)。 */
export function flagNumbers(el, nums) {
  if (!nums.length) return;
  const want = new Set(nums);
  const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
  const nodes = [];
  while (walker.nextNode()) nodes.push(walker.currentNode);
  const re = /\d+(?:[.,:/]\d+)*/g;
  for (const node of nodes) {
    const s = node.data;
    let m, last = 0, hit = false;
    const frag = document.createDocumentFragment();
    re.lastIndex = 0;
    while ((m = re.exec(s))) {
      const key = m[0].replace(/,(?=\d{3}(?!\d))/g, '');
      if (!want.has(key)) continue;
      hit = true;
      frag.append(s.slice(last, m.index));
      const mark = document.createElement('span');
      mark.className = 'numflag';
      mark.textContent = m[0];
      frag.append(mark);
      last = m.index + m[0].length;
    }
    if (hit) { frag.append(s.slice(last)); node.replaceWith(frag); }
  }
}
