// 横条图的两个小部件:barRows 画一组行(名字 | 条 | 数值),axis 画刻度。
// 条长 = v(0~1);数值文字原样用调用方给的字符串(都来自事实清单),这里不做任何换算。
import { esc } from './common.js';

export function barRows(rows, opts = {}) {
  const ticks = [0, 25, 50, 75, 100].map((p) => `<i style="left:${p}%"></i>`).join('');
  return `<div class="plot"><div class="gl">${ticks}</div>${rows.map((r) => `
    <div class="row${r.me ? ' me' : ''}${r.base ? ' base' : ''}${r.flag ? ' flag' : ''}${opts.small ? ' small' : ''}">
      <div class="nm"><span class="dot"></span><span>${esc(r.name)}</span></div>
      <div class="tr">${r.v > 0 ? `<div class="bar" style="width:${(r.v * 100).toFixed(2)}%"></div>` : '<div class="zero"></div>'}</div>
      <div class="val"><b>${esc(r.big)}</b>${r.note ? `<small>${esc(r.note)}</small>` : ''}</div>
    </div>`).join('')}</div>`;
}

export function axis(labels, cap) {
  const n = labels.length - 1;
  return `<div class="plot"><div class="axis"><span></span><div class="ticks">${labels.map((l, i) => `<span style="left:${(i / n) * 100}%">${esc(l)}</span>`).join('')}</div><span class="cap">${esc(cap)}</span></div></div>`;
}
