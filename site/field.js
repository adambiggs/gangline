/* SPDX-License-Identifier: Apache-2.0 */
(() => {
  const canvas = document.getElementById('field');
  if (!canvas) return;
  if (!window.LivingField) throw new Error('Living Field bundle did not load');
  window.LivingField.createField(canvas, { preset: window.LivingField.presets.gangline, persist: 'session' });
})();
