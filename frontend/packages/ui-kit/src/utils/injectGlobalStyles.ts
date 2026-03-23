/**
 * 将 CSS 样式注入到 document.head，确保每个 id 只注入一次。
 *
 * 用途：将 @keyframes 等动画规则注入全局，避免 qiankun experimentalStyleIsolation
 * 将子应用 <style> 标签内的内容进行沙箱隔离，导致 @keyframes 无法被 animation 属性引用。
 *
 * document.head 中的样式不受 qiankun 沙箱管理，因此全局可访问。
 */

const injected = new Set<string>();

export function injectGlobalStyles(id: string, css: string): void {
  if (typeof document === 'undefined') return;
  if (injected.has(id)) return;
  if (document.getElementById(id)) {
    injected.add(id);
    return;
  }
  injected.add(id);
  const style = document.createElement('style');
  style.id = id;
  style.textContent = css;
  document.head.appendChild(style);
}
