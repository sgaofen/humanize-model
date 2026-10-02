// 提示词拼接。字符串本身(instr / sep)由启动器 /app/config 下发,
// 启动器那份又被单测钉在 humanizer/promptfmt.py 的指纹上 —— 三处只有一个事实源。
import { pyStrip } from './text.js';

/** prompt = INSTR + "\n\n" + draft.strip() + "\n\n### Rewritten:\n\n" */
export function buildPrompt(cfg, draft) {
  return cfg.instr + '\n\n' + pyStrip(draft) + cfg.sep;
}

/** 用 probe(启动器拼的 BuildPrompt("X"))逐字比对,防止两边漂移。 */
export function selfCheck(cfg) {
  return typeof cfg.probe === 'string' && buildPrompt(cfg, 'X') === cfg.probe;
}

/** n_predict = clamp(草稿 token × 2.5, 256, 2048) */
export function nPredictFor(cfg, draftTokens) {
  const r = cfg.n_predict || { factor: 2.5, min: 256, max: 2048 };
  return Math.min(r.max, Math.max(r.min, Math.ceil(draftTokens * r.factor)));
}
