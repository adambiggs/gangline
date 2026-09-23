// SPDX-License-Identifier: Apache-2.0
(() => {
  const video = document.querySelector('#demo-video');
  const source = document.querySelector('#demo-source');
  if (!video || !source) return;
  const media = matchMedia('(prefers-color-scheme: dark)');
  const version = new URL(source.src).search;
  let initialized = false;
  const apply = () => {
    const first = !initialized;
    initialized = true;
    const dark = document.documentElement.dataset.theme === 'dark' ||
      (!document.documentElement.dataset.theme && media.matches);
    const suffix = dark ? '' : '-light';
    const src = `demo${suffix}.mp4${version}`;
    if (source.getAttribute('src') === src) return;
    const time = video.currentTime;
    const playing = !video.paused || (first && video.autoplay);
    source.src = src;
    video.poster = `demo-poster${suffix}.jpg${version}`;
    video.onloadedmetadata = () => {
      video.currentTime = Math.min(time, video.duration);
      if (playing) video.play().catch(() => {});
      else video.pause();
    };
    video.load();
  };
  new MutationObserver(apply).observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
  media.addEventListener('change', apply);
  apply();
})();
