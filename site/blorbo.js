/* SPDX-License-Identifier: Apache-2.0 */
(() => {
  const canvas = document.getElementById('blorbo');
  if (!canvas) return;
  if (!window.Blorbo) throw new Error('Blorbo bundle did not load');
  window.Blorbo.createBlorbo(canvas, { preset: window.Blorbo.presets.gangline, persist: 'session' });
})();
