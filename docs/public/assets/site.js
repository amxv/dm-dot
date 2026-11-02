(() => {
  document.documentElement.classList.add('has-js');
  const replay = document.querySelector('[data-replay]');
  const diagram = document.querySelector('.hero-diagram');
  if (replay && diagram) {
    replay.addEventListener('click', () => {
      diagram.classList.remove('playing');
      void diagram.offsetWidth;
      diagram.classList.add('playing');
    });
    diagram.classList.add('playing');
  }

  const tabs = [...document.querySelectorAll('[data-code-tab]')];
  const panels = [...document.querySelectorAll('[data-code-content]')];
  const fileLabel = document.querySelector('[data-file-label]');
  function activateTab(name) {
    for (const tab of tabs) {
      const active = tab.dataset.codeTab === name;
      tab.classList.toggle('active', active);
      tab.setAttribute('aria-selected', String(active));
      tab.tabIndex = active ? 0 : -1;
    }
    for (const panel of panels) panel.hidden = panel.dataset.codeContent !== name;
    if (fileLabel) fileLabel.textContent = name === 'mcp' ? 'send_to_dot' : 'POST /v1/messages';
  }
  for (const tab of tabs) {
    tab.addEventListener('click', () => activateTab(tab.dataset.codeTab));
    tab.addEventListener('keydown', (event) => {
      if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
      event.preventDefault();
      const index = tabs.indexOf(tab);
      const next = tabs[(index + (event.key === 'ArrowRight' ? 1 : tabs.length - 1)) % tabs.length];
      activateTab(next.dataset.codeTab);
      next.focus();
    });
  }

  const copy = document.querySelector('[data-copy-code]');
  if (copy) {
    let reset;
    copy.addEventListener('click', async () => {
      const active = panels.find((panel) => !panel.hidden);
      if (!active) return;
      const value = active.querySelector('code')?.textContent ?? '';
      try {
        await navigator.clipboard.writeText(value);
        const label = copy.querySelector('span');
        label.textContent = 'Copied';
        clearTimeout(reset);
        reset = setTimeout(() => { label.textContent = 'Copy'; }, 1600);
      } catch {
        const label = copy.querySelector('span');
        label.textContent = 'Select to copy';
        clearTimeout(reset);
        reset = setTimeout(() => { label.textContent = 'Copy'; }, 1800);
      }
    });
  }

  const reveals = [...document.querySelectorAll('.process-card.reveal, .section-head.reveal')];
  if ('IntersectionObserver' in window && !window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
    const observer = new IntersectionObserver((entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          entry.target.classList.add('on-screen');
          observer.unobserve(entry.target);
        }
      }
    }, { threshold: 0.1 });
    reveals.forEach((element) => observer.observe(element));
  } else reveals.forEach((element) => element.classList.add('on-screen'));
})();
