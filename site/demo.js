// SPDX-License-Identifier: Apache-2.0
(() => {
  const video = document.querySelector('#demo-video');
  const source = document.querySelector('#demo-source');
  if (!video || !source) return;
  const controls = document.querySelector('.demo-controls');
  const toggle = document.querySelector('#demo-toggle');
  const seek = document.querySelector('#demo-seek');
  const timeLabel = document.querySelector('#demo-time');
  const format = n => `${Math.floor(n / 60)}:${String(Math.floor(n % 60)).padStart(2, '0')}`;
  const update = () => {
    const duration = Number.isFinite(video.duration) ? video.duration : 0;
    toggle.textContent = video.paused ? 'Play' : 'Pause';
    seek.max = duration || 1;
    seek.value = video.currentTime;
    timeLabel.value = `${format(video.currentTime)} / ${format(duration)}`;
  };
  toggle.addEventListener('click', () => {
    if (video.paused) video.play().catch(() => update());
    else video.pause();
  });
  seek.addEventListener('input', () => { video.currentTime = Number(seek.value); });
  // The recording is a full-width terminal; full screen shows it at native size.
  const fullscreen = document.querySelector('#demo-fullscreen');
  const enter = () => {
    if (video.requestFullscreen) video.requestFullscreen().catch(() => {});
    else if (video.webkitEnterFullscreen) video.webkitEnterFullscreen();
  };
  if (video.requestFullscreen || video.webkitEnterFullscreen) {
    fullscreen.hidden = false;
    fullscreen.addEventListener('click', enter);
    video.addEventListener('click', enter);
    document.addEventListener('fullscreenchange', () => { video.controls = document.fullscreenElement === video; });
  }
  for (const event of ['timeupdate', 'loadedmetadata', 'play', 'pause']) video.addEventListener(event, update);
  controls.hidden = false;
  video.controls = false;
  if (matchMedia('(prefers-reduced-motion: reduce)').matches) {
    video.autoplay = false;
    video.pause();
  }
  update();
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
