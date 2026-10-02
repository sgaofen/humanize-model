// 网页纯逻辑的单测(不需要浏览器):node devtools/webtest.mjs 或 bun devtools/webtest.mjs
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { pyStrip, countText, numberCheck } from '../web/js/text.js';
import { buildPrompt, nPredictFor, selfCheck } from '../web/js/prompt.js';
import { tokenize, diffTokens } from '../web/js/diff.js';

let fails = 0;
const ok = (cond, msg) => { if (!cond) { fails++; console.error('✗', msg); } else console.log('✓', msg); };

// 1. strip 和 Python 一致(期望值由 python3 str.strip() 生成)
const STRIP = [[" \n\t\u3000\u001c Hello world.\u00a0\n\n", "Hello world."], ["\ufeffx\ufeff", "\ufeffx\ufeff"], ["\u200bzero\u200b", "\u200bzero\u200b"], ["\u0085a\u001f", "a"], ["a\n\n b", "a\n\n b"], ["\u2028x\u2029", "x"], ["\u000bv\f", "v"]];
for (const [inp, want] of STRIP) ok(pyStrip(inp) === want, `pyStrip(${JSON.stringify(inp)})`);

// 2. 提示词指纹 = promptfmt.py 的 fingerprint()
const INSTR = 'Rewrite the text below so it reads like a person wrote it, not a language model.\n\nReorganize it as you see fit. Vary sentence length on purpose. Cut hedging,\nthroat-clearing, and any sentence that only announces what comes next.\nPrefer the concrete word over the abstract one. It is fine to sound uneven.\n\nEvery fact, number, unit, date, name and quotation must survive unchanged.';
const cfg = { instr: INSTR, sep: '\n\n### Rewritten:\n\n', n_predict: { factor: 2.5, min: 256, max: 2048 } };
const fp = createHash('sha256').update(buildPrompt(cfg, 'X')).digest('hex').slice(0, 16);
ok(fp === 'cc51d66b4c593fbe', `提示词指纹 ${fp} == promptfmt.py cc51d66b4c593fbe`);
ok(selfCheck({ ...cfg, probe: buildPrompt(cfg, 'X') }) && !selfCheck({ ...cfg, probe: 'nope' }), 'selfCheck 正反两面');
ok(!buildPrompt(cfg, 'a').includes('###\n') && buildPrompt(cfg, '  a  ').endsWith('\n\na\n\n### Rewritten:\n\n'), '草稿两端 strip、分隔符正确');

// 3. n_predict = clamp(ceil(n × 2.5), 256, 2048)
ok(nPredictFor(cfg, 10) === 256 && nPredictFor(cfg, 200) === 500 && nPredictFor(cfg, 101) === 256 && nPredictFor(cfg, 103) === 258 && nPredictFor(cfg, 5000) === 2048, 'n_predict 规则');

// 4. 字数
const c1 = countText('Hello, world! 你好世界');
ok(c1.latin === 2 && c1.cjk === 4 && c1.units === 6, `字数统计 ${JSON.stringify(c1)}`);

// 5. 差异:英文按词、中文按字
{
  const a = tokenize('The quick brown fox jumps over the lazy dog.');
  const b = tokenize('The quick red fox leaps over the lazy dog.');
  const d = diffTokens(a, b);
  const insB = b.filter((x) => !x.ws && d.b[x.ci]).map((x) => x.t);
  const delA = a.filter((x) => !x.ws && d.a[x.ci]).map((x) => x.t);
  ok(JSON.stringify(insB) === '["red","leaps"]' && JSON.stringify(delA) === '["brown","jumps"]', `英文词级差异 +${insB} -${delA}`);
}
{
  const a = tokenize('我今天去北京开会。');
  const b = tokenize('我明天去北京开会。');
  const d = diffTokens(a, b);
  const insB = b.filter((x) => !x.ws && d.b[x.ci]).map((x) => x.t).join('');
  ok(insB === '明', `中文字级差异 +${insB}`);
}
{
  // 语义清理:深度改写时不应留下孤零零的 "the"
  const a = tokenize('It was a great salary with smart colleagues and the free meals were nice.');
  const b = tokenize('Pay was huge, people were sharp, the food cost nothing.');
  const d = diffTokens(a, b);
  const kept = b.filter((x) => !x.ws && !d.b[x.ci]).map((x) => x.t);
  ok(kept.length <= 2, `深改写不留碎片,保留的 token: ${JSON.stringify(kept)}`);
}
{
  const same = tokenize('完全相同的一句话。');
  const d = diffTokens(same, tokenize('完全相同的一句话。'));
  ok(d.ratio === 0 && !d.a.some(Boolean), '相同文本无差异');
}

// 6. 数字核对
{
  const r = numberCheck('From $1,850 to 3,200 users on 6:40, 60 minutes, 2024.', 'From 1850 dollars to 3200 users at 6:40 — an hour — in 2024');
  ok(JSON.stringify(r.missing) === '["60"]' && r.total === 5, `数字核对 ${JSON.stringify(r)}`);
  const r2 = numberCheck('晚上8点到10点半', '晚上8点到10:30');
  ok(r2.missing.length === 0, '10 点半 → 10:30 不算丢');
}

// 7. 用真实夹具跑一遍差异,确认不炸、比例合理
const fx = JSON.parse(readFileSync(new URL('./fixtures/samples.json', import.meta.url), 'utf8'));
for (const s of fx.samples) {
  const d = diffTokens(tokenize(s.draft), tokenize(s.output));
  const r = numberCheck(s.draft, s.output);
  ok(d && d.ratio > 0.2 && d.ratio < 0.95, `${s.id} 改动比例 ${(d.ratio * 100).toFixed(0)}%,数字缺 ${JSON.stringify(r.missing)}`);
}

if (fails) { console.error(`\n${fails} 项失败`); process.exit(1); }
console.log('\n全部通过');
