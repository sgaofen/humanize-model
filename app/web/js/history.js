// 历史记录:只存浏览器 localStorage(按 "127.0.0.1:端口" 隔离,所以启动器尽量固定端口)。
const KEY = 'hz.history.v1';
const MAX = 100;

export function loadHistory() {
  try {
    const v = JSON.parse(localStorage.getItem(KEY) || '[]');
    return Array.isArray(v) ? v : [];
  } catch {
    return [];
  }
}

function save(list) {
  for (let n = list.length; n > 0; n = Math.floor(n / 2)) {
    try {
      localStorage.setItem(KEY, JSON.stringify(list.slice(0, n)));
      return;
    } catch {
      // 配额满了:丢掉老的一半再试
    }
  }
}

export function addHistory(entry) {
  const list = loadHistory();
  list.unshift(entry);
  save(list.slice(0, MAX));
}

export function removeHistory(id) {
  save(loadHistory().filter((x) => x.id !== id));
}

export function clearHistory() {
  try { localStorage.removeItem(KEY); } catch { /* 无痕模式等 */ }
}

export const store = {
  get(k, def = null) { try { const v = localStorage.getItem('hz.' + k); return v === null ? def : v; } catch { return def; } },
  set(k, v) { try { localStorage.setItem('hz.' + k, v); } catch { /* 忽略 */ } },
};
