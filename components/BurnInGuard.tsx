"use client";

import { createSignal, onCleanup, onMount } from "@barefootjs/client";

export function BurnInGuard() {
  const [visible, setVisible] = createSignal(false);

  onMount(() => {
    let dismiss = 0;
    const show = () => {
      setVisible(true);
      dismiss = window.setTimeout(() => setVisible(false), 36 * 1000);
    };
    const first = window.setTimeout(show, 5 * 60 * 1000);
    const cycle = window.setInterval(show, 15 * 60 * 1000);
    onCleanup(() => {
      clearTimeout(first);
      clearTimeout(dismiss);
      clearInterval(cycle);
    });
  });

  return (
    <section
      className={`burnin-guard ${visible() ? "is-visible" : ""}`}
      role="status"
      aria-label="画面保護アニメーション"
      aria-hidden={!visible()}
    >
      <svg className="burnin-symbols" aria-hidden="true">
        <symbol id="picto-home" viewBox="0 0 100 100">
          <path d="M12 47 50 15l38 32v38H61V61H39v24H12Z" />
        </symbol>
        <symbol id="picto-eye" viewBox="0 0 100 100">
          <path d="M7 50s16-25 43-25 43 25 43 25-16 25-43 25S7 50 7 50Z" />
          <circle cx="50" cy="50" r="12" />
        </symbol>
        <symbol id="picto-person" viewBox="0 0 100 100">
          <circle cx="50" cy="18" r="11" />
          <path d="m50 34-3 28-18 25m18-25 21 24M47 44 25 59m22-15 23 12" />
        </symbol>
        <symbol id="picto-signal" viewBox="0 0 100 100">
          <circle cx="50" cy="76" r="6" />
          <path d="M35 61a21 21 0 0 1 30 0M22 47a40 40 0 0 1 56 0M9 33a58 58 0 0 1 82 0" />
        </symbol>
        <symbol id="picto-sun" viewBox="0 0 100 100">
          <circle cx="50" cy="50" r="19" />
          <path d="M50 6v15m0 58v15M6 50h15m58 0h15M19 19l11 11m40 40 11 11m0-62L70 30M30 70 19 81" />
        </symbol>
        <symbol id="picto-bolt" viewBox="0 0 100 100">
          <path d="M58 5 20 58h27l-5 37 38-55H54Z" />
        </symbol>
        <symbol id="picto-arrow" viewBox="0 0 100 100">
          <path d="M8 50h74M57 25l25 25-25 25" />
        </symbol>
      </svg>

      <div className="burnin-field" aria-hidden="true">
        {[
          ["home", "B"],
          ["eye", "EYE"],
          ["person", "K"],
          ["signal", "///"],
          ["sun", "LIGHT"],
          ["bolt", "+"],
          ["arrow", "SHIFT"],
          ["eye", "00"],
          ["signal", "K"],
          ["person", "MOVE"],
          ["bolt", "B"],
          ["sun", "▲"],
        ].map(([icon, label], index) => (
          <figure className={`burnin-picto picto-${icon} scatter-${index + 1}`} key={index}>
            <svg viewBox="0 0 100 100">
              <use href={`#picto-${icon}`} />
            </svg>
            <figcaption>{label}</figcaption>
          </figure>
        ))}
        <b className="burnin-type type-a">B.K</b>
        <b className="burnin-type type-b">→→→</b>
        <b className="burnin-type type-c">0101</b>
      </div>
    </section>
  );
}
