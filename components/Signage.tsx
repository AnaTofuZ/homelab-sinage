"use client";

import { createSignal, onCleanup, onMount } from "@barefootjs/client";

type Weather = {
  place: string;
  temperature: number;
  apparent: number;
  code: number;
  condition: string;
  high: number;
  low: number;
  rain: number;
};
type Forecast = {
  date: string;
  weekday: string;
  code: number;
  condition: string;
  high: number;
  low: number;
  rain: number;
};
type Event = { id: string; title: string; time: string; location: string; allDay: boolean };
type News = { title: string; url: string; published: string; summary: string };
type Attendance = {
  available: boolean;
  state: string;
  workedSeconds: number;
  targetSeconds: number;
  dayDifference: number;
  monthDifference: number;
  projectedSeconds: number;
};
type Dashboard = {
  generatedAt: string;
  weather: Weather;
  forecast: Forecast[];
  events: Event[];
  news: News[];
  attendance: Attendance;
  warnings: string[];
};

const empty: Dashboard = {
  generatedAt: "",
  weather: {
    place: "KOFU",
    temperature: 0,
    apparent: 0,
    code: 0,
    condition: "接続中",
    high: 0,
    low: 0,
    rain: 0,
  },
  forecast: [],
  events: [],
  news: [],
  attendance: {
    available: false,
    state: "offline",
    workedSeconds: 0,
    targetSeconds: 28800,
    dayDifference: 0,
    monthDifference: 0,
    projectedSeconds: 0,
  },
  warnings: [],
};

function duration(seconds: number, signed = false) {
  const sign = signed ? (seconds >= 0 ? "+" : "−") : "";
  const minutes = Math.floor(Math.abs(seconds) / 60);
  return `${sign}${Math.floor(minutes / 60)}:${String(minutes % 60).padStart(2, "0")}`;
}

function weatherGlyph(code: number) {
  if (code === 0) return "○";
  if (code <= 3) return "◐";
  if (code <= 48) return "≋";
  if (code <= 67 || (code >= 80 && code <= 82)) return "///";
  if (code <= 86) return "＊";
  return "ϟ";
}

export function Signage() {
  const [data, setData] = createSignal<Dashboard>(empty);
  const [now, setNow] = createSignal(new Date());
  const [busy, setBusy] = createSignal(false);
  const [error, setError] = createSignal("");
  const [newsTakeover, setNewsTakeover] = createSignal(false);
  const [newsIndex, setNewsIndex] = createSignal(0);

  const reload = async () => {
    try {
      const response = await fetch("/api/dashboard");
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      setData(await response.json());
      setError("");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "更新できません");
    }
  };

  const action = async (name: string) => {
    setBusy(true);
    try {
      const response = await fetch(`/api/attendance/${name}`, { method: "POST" });
      const body = await response.json();
      if (!response.ok) throw new Error(body.error ?? `HTTP ${response.status}`);
      setData({ ...data(), attendance: body });
      setError("");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "操作できません");
    } finally {
      setBusy(false);
    }
  };

  onMount(() => {
    void reload();
    const clock = window.setInterval(() => setNow(new Date()), 1000);
    const refresh = window.setInterval(() => void reload(), 5 * 60 * 1000);
    let dismissNews = 0;
    const showNews = () => {
      if (!data().news.length) return;
      setNewsTakeover(true);
      clearTimeout(dismissNews);
      dismissNews = window.setTimeout(() => {
        setNewsTakeover(false);
        const count = data().news.length;
        setNewsIndex(count ? (newsIndex() + 1) % count : 0);
      }, 24 * 1000);
    };
    const firstNews = window.setTimeout(showNews, 30 * 1000);
    const newsCycle = window.setInterval(showNews, 3 * 60 * 1000);
    onCleanup(() => {
      clearInterval(clock);
      clearInterval(refresh);
      clearInterval(newsCycle);
      clearTimeout(firstNews);
      clearTimeout(dismissNews);
    });
  });

  const stateLabel = () =>
    ({ working: "勤務中", break: "休憩中", off: "退勤済" })[data().attendance.state] ?? "未接続";
  const shownWorked = () =>
    data().attendance.workedSeconds +
    (data().attendance.state === "working"
      ? Math.max(0, (Date.now() - Date.parse(data().generatedAt)) / 1000)
      : 0);
  const currentNews = () => data().news[newsIndex() % data().news.length];

  return (
    <main className="signage">
      <div className="scanline" aria-hidden="true" />
      <header className="topbar">
        <p className="brand">
          <i /> HOME SIGNAL <span>甲府 / PRIVATE NETWORK</span>
        </p>
        <p className="system">
          {error() ? `DEGRADED · ${error()}` : "SYSTEM NORMAL"} <b />
        </p>
      </header>

      <section className="hero panel">
        <div className="clock">
          <p className="eyebrow">LOCAL TIME · JST</p>
          <time>
            {/* @client */ String(now().getHours()).padStart(2, "0")}
            <em>:</em>
            {/* @client */ String(now().getMinutes()).padStart(2, "0")}
          </time>
          <p className="seconds">{/* @client */ String(now().getSeconds()).padStart(2, "0")}</p>
          <p className="date">
            {
              /* @client */ new Intl.DateTimeFormat("ja-JP", {
                year: "numeric",
                month: "2-digit",
                day: "2-digit",
                weekday: "long",
              }).format(now())
            }
          </p>
        </div>
        <div className="weather-mark" aria-label={data().weather.condition}>
          {/* @client */ weatherGlyph(data().weather.code)}
        </div>
        <div className="weather-now">
          <p className="eyebrow">WEATHER · {data().weather.place}</p>
          <p className="temperature">
            {data().weather.temperature.toFixed(1)}
            <small>°C</small>
          </p>
          <p className="condition">
            {data().weather.condition} <span>体感 {data().weather.apparent.toFixed(1)}°</span>
          </p>
          <dl>
            <div>
              <dt>HIGH</dt>
              <dd>{data().weather.high.toFixed(0)}°</dd>
            </div>
            <div>
              <dt>LOW</dt>
              <dd>{data().weather.low.toFixed(0)}°</dd>
            </div>
            <div>
              <dt>RAIN</dt>
              <dd>{data().weather.rain}%</dd>
            </div>
          </dl>
        </div>
      </section>

      <section className="content-grid">
        <article className="schedule panel reveal">
          <header>
            <span>01</span>
            <div>
              <p className="eyebrow">TODAY / NEXT</p>
              <h2>予定</h2>
            </div>
            <b>{String(data().events.length).padStart(2, "0")}</b>
          </header>
          <ol>
            {data().events.length ? (
              data().events.map((event) => (
                <li key={event.id}>
                  <time>{event.time}</time>
                  <i />
                  <div>
                    <strong>{event.title}</strong>
                    {event.location && <small>{event.location}</small>}
                  </div>
                </li>
              ))
            ) : (
              <li className="empty">直近の予定はありません</li>
            )}
          </ol>
        </article>

        <article className="forecast panel reveal">
          <header>
            <span>02</span>
            <div>
              <p className="eyebrow">4 DAYS</p>
              <h2>予報</h2>
            </div>
          </header>
          <div className="forecast-list">
            {data().forecast.map((day, index) => (
              <div className="forecast-day" key={day.date}>
                <p>{index === 0 ? "TODAY" : day.weekday}</p>
                <strong>{/* @client */ weatherGlyph(day.code)}</strong>
                <div>
                  <b>{day.high.toFixed(0)}°</b>
                  <span>{day.low.toFixed(0)}°</span>
                </div>
                <small>{day.rain}%</small>
              </div>
            ))}
          </div>
        </article>

        <article className="attendance panel reveal">
          <header>
            <span>03</span>
            <div>
              <p className="eyebrow">FLEX TIME</p>
              <h2>勤怠</h2>
            </div>
            <b className={data().attendance.available ? "online" : ""}>
              {/* @client */ stateLabel()}
            </b>
          </header>
          {data().attendance.available ? (
            <>
              <div className="worked">
                <p>WORKED TODAY</p>
                <strong>{/* @client */ duration(shownWorked())}</strong>
                <small>/ {/* @client */ duration(data().attendance.targetSeconds)}</small>
              </div>
              <div className="balance">
                <p>
                  DAY{" "}
                  <b>
                    {/* @client */ duration(shownWorked() - data().attendance.targetSeconds, true)}
                  </b>
                </p>
                <p>
                  MONTH <b>{/* @client */ duration(data().attendance.monthDifference, true)}</b>
                </p>
              </div>
              <div className="actions">
                {data().attendance.state === "off" && (
                  <button disabled={busy()} onClick={() => action("clock-in")}>
                    出勤
                  </button>
                )}
                {data().attendance.state === "working" && (
                  <>
                    <button disabled={busy()} onClick={() => action("break-start")}>
                      休憩
                    </button>
                    <button disabled={busy()} onClick={() => action("clock-out")}>
                      退勤
                    </button>
                  </>
                )}
                {data().attendance.state === "break" && (
                  <>
                    <button disabled={busy()} onClick={() => action("break-end")}>
                      再開
                    </button>
                    <button disabled={busy()} onClick={() => action("clock-out")}>
                      退勤
                    </button>
                  </>
                )}
              </div>
            </>
          ) : (
            <div className="empty attendance-empty">FLEX_TIMER_URL を設定すると表示されます</div>
          )}
        </article>
      </section>

      <footer className="newsbar">
        <div className="news-label">
          <span>04</span> NEWS
        </div>
        <div className="ticker">
          <div>
            {data().news.map((item) => (
              <span key={item.url}>
                <time>{item.published}</time>
                {item.title}
                <i>◆</i>
              </span>
            ))}
          </div>
        </div>
        <p>
          {/* @client */ now().toLocaleTimeString("ja-JP", { hour: "2-digit", minute: "2-digit" })}
        </p>
      </footer>

      {newsTakeover() && data().news.length > 0 && (
        <section className="news-takeover" role="status" aria-label="ニュース">
          <header>
            <p>
              <i /> HOME SIGNAL / NEWS
            </p>
            <time>
              {
                /* @client */ now().toLocaleTimeString("ja-JP", {
                  hour: "2-digit",
                  minute: "2-digit",
                })
              }
            </time>
          </header>
          <div className="news-takeover-title">
            <p>INFORMATION DISPLAY · KOFU</p>
            <h2>ニュース</h2>
          </div>
          <article className="news-takeover-story">
            <b>{String((newsIndex() % data().news.length) + 1).padStart(2, "0")}</b>
            <div>
              <time>{currentNews().published || "--:--"}</time>
              <h3>{currentNews().title}</h3>
              <p>{currentNews().summary}</p>
            </div>
          </article>
          <footer>
            <span>
              {String((newsIndex() % data().news.length) + 1).padStart(2, "0")} /{" "}
              {String(data().news.length).padStart(2, "0")} · 更新{" "}
              {data().generatedAt.slice(11, 16)}
            </span>
            <p>このあと通常画面へ戻ります</p>
          </footer>
        </section>
      )}
    </main>
  );
}
