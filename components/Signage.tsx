"use client";

import { createSignal, onCleanup, onMount } from "@barefootjs/client";
import { BurnInGuard } from "./BurnInGuard";

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
type HourlyWeather = {
  time: string;
  code: number;
  condition: string;
  temperature: number;
  rain: number;
  precip: number;
  wind: number;
};
type WeatherAlert = { level: string; title: string; detail: string };
type Event = {
  id: string;
  title: string;
  startsAt: string;
  time: string;
  endTime: string;
  day: "today" | "tomorrow";
  location: string;
  allDay: boolean;
};
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
  hourly: HourlyWeather[];
  alerts: WeatherAlert[];
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
  hourly: [],
  alerts: [],
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
  const [attendanceUpdatedAt, setAttendanceUpdatedAt] = createSignal(Date.now());
  const [busy, setBusy] = createSignal(false);
  const [newsTakeover, setNewsTakeover] = createSignal(false);
  const [newsPage, setNewsPage] = createSignal(0);
  const [scheduleTakeover, setScheduleTakeover] = createSignal(false);
  const [schedulePage, setSchedulePage] = createSignal(0);
  const [attendanceTakeover, setAttendanceTakeover] = createSignal(false);
  const [eventAlert, setEventAlert] = createSignal<Event | null>(null);
  const [soundReady, setSoundReady] = createSignal(false);
  let audioContext: AudioContext | null = null;

  const enableSound = async () => {
    audioContext ??= new AudioContext();
    await audioContext.resume();
    setSoundReady(audioContext.state === "running");
  };

  const playJackpotAlert = () => {
    if (!audioContext || audioContext.state !== "running") return;
    const started = audioContext.currentTime;
    [0, 0.14, 0.28, 0.52, 0.66, 0.8, 1.04, 1.18, 1.32].forEach((offset, index) => {
      const oscillator = audioContext!.createOscillator();
      const gain = audioContext!.createGain();
      oscillator.type = index < 3 ? "square" : "sawtooth";
      oscillator.frequency.setValueAtTime(330 * 2 ** ((index % 6) / 12), started + offset);
      oscillator.frequency.exponentialRampToValueAtTime(
        660 * 2 ** ((index % 6) / 12),
        started + offset + 0.11,
      );
      gain.gain.setValueAtTime(0.0001, started + offset);
      gain.gain.exponentialRampToValueAtTime(0.13, started + offset + 0.015);
      gain.gain.exponentialRampToValueAtTime(0.0001, started + offset + 0.12);
      oscillator.connect(gain).connect(audioContext!.destination);
      oscillator.start(started + offset);
      oscillator.stop(started + offset + 0.13);
    });
  };

  const reload = async () => {
    try {
      const response = await fetch("/api/dashboard");
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const body = await response.json();
      setAttendanceUpdatedAt(Date.parse(body.generatedAt));
      setData(body);
    } catch (reason) {
      console.error(reason);
    }
  };

  const action = async (name: string) => {
    setBusy(true);
    try {
      const response = await fetch(`/api/attendance/${name}`, { method: "POST" });
      const body = await response.json();
      if (!response.ok) throw new Error(body.error ?? `HTTP ${response.status}`);
      setAttendanceUpdatedAt(Date.now());
      setData({ ...data(), attendance: body });
    } catch (reason) {
      console.error(reason);
    } finally {
      setBusy(false);
    }
  };

  onMount(() => {
    void reload();
    const clock = window.setInterval(() => setNow(new Date()), 1000);
    let dismissEventAlert = 0;
    const announced = new Set<string>();
    const checkEventAlert = () => {
      if (eventAlert()) return;
      const current = Date.now();
      const event = data().events.find((candidate) => {
        if (candidate.allDay || announced.has(candidate.id)) return false;
        const elapsed = current - Date.parse(candidate.startsAt);
        return elapsed >= 0 && elapsed < 60 * 1000;
      });
      if (!event) return;
      announced.add(event.id);
      setNewsTakeover(false);
      setScheduleTakeover(false);
      setAttendanceTakeover(false);
      setEventAlert(event);
      playJackpotAlert();
      dismissEventAlert = window.setTimeout(() => setEventAlert(null), 18 * 1000);
    };
    const eventAlertClock = window.setInterval(checkEventAlert, 1000);
    const refresh = window.setInterval(() => void reload(), 5 * 60 * 1000);
    let dismissNews = 0;
    const showNews = () => {
      if (!data().news.length || scheduleTakeover() || attendanceTakeover()) return;
      setNewsTakeover(true);
      clearTimeout(dismissNews);
      dismissNews = window.setTimeout(() => {
        setNewsTakeover(false);
        const pages = Math.ceil(data().news.length / 3);
        setNewsPage(pages ? (newsPage() + 1) % pages : 0);
      }, 24 * 1000);
    };
    const firstNews = window.setTimeout(showNews, 30 * 1000);
    const newsCycle = window.setInterval(showNews, 3 * 60 * 1000);
    let dismissSchedule = 0;
    const showSchedule = () => {
      if (!data().events.length || newsTakeover() || attendanceTakeover()) return;
      setScheduleTakeover(true);
      clearTimeout(dismissSchedule);
      dismissSchedule = window.setTimeout(() => {
        setScheduleTakeover(false);
        const pages = Math.ceil(data().events.length / 4);
        setSchedulePage(pages ? (schedulePage() + 1) % pages : 0);
      }, 24 * 1000);
    };
    const firstSchedule = window.setTimeout(showSchedule, 90 * 1000);
    const scheduleCycle = window.setInterval(showSchedule, 3 * 60 * 1000);
    let dismissAttendance = 0;
    const showAttendance = () => {
      if (!data().attendance.available || newsTakeover() || scheduleTakeover()) return;
      setAttendanceTakeover(true);
      clearTimeout(dismissAttendance);
      dismissAttendance = window.setTimeout(() => setAttendanceTakeover(false), 24 * 1000);
    };
    const firstAttendance = window.setTimeout(showAttendance, 150 * 1000);
    const attendanceCycle = window.setInterval(showAttendance, 3 * 60 * 1000);
    onCleanup(() => {
      clearInterval(clock);
      clearInterval(eventAlertClock);
      clearInterval(refresh);
      clearInterval(newsCycle);
      clearInterval(scheduleCycle);
      clearInterval(attendanceCycle);
      clearTimeout(firstNews);
      clearTimeout(dismissNews);
      clearTimeout(firstSchedule);
      clearTimeout(dismissSchedule);
      clearTimeout(firstAttendance);
      clearTimeout(dismissAttendance);
      clearTimeout(dismissEventAlert);
      void audioContext?.close();
    });
  });

  const stateLabel = () =>
    ({ working: "勤務中", break: "休憩中", off: "退勤済" })[data().attendance.state] ?? "未接続";
  const shownWorked = () =>
    data().attendance.workedSeconds +
    (data().attendance.state === "working"
      ? Math.max(0, (now().getTime() - attendanceUpdatedAt()) / 1000)
      : 0);
  const newsPageStart = () => (newsPage() % Math.ceil(data().news.length / 3)) * 3;
  const schedulePageStart = () => (schedulePage() % Math.ceil(data().events.length / 4)) * 4;
  const eventTime = (event: Event) =>
    event.endTime ? event.time + "–" + event.endTime : event.time;
  const alertLevel = () =>
    data().alerts.some((alert) => alert.level === "emergency")
      ? "emergency"
      : data().alerts.some((alert) => alert.level === "danger")
        ? "danger"
        : data().alerts.some((alert) => alert.level === "warning")
          ? "warning"
          : "advisory";

  return (
    <main className={`signage ${data().alerts.length ? "has-weather-alert" : ""}`}>
      <div className="scanline" aria-hidden="true" />

      {!soundReady() && (
        <button
          className="sound-enable"
          onClick={() => void enableSound()}
          aria-label="予定通知音を有効にする"
        >
          SOUND OFF · タップで予定通知音を有効化
        </button>
      )}

      {data().alerts.length > 0 && (
        <aside className={`weather-alert ${alertLevel()}`} role="alert">
          <p>WEATHER ALERT · 甲府市</p>
          <strong>
            {data()
              .alerts.map((alert) => alert.title)
              .join(" / ")}
          </strong>
          <span>{data().alerts[0].detail}</span>
        </aside>
      )}

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
        <div className="weather-now">
          <p className="eyebrow">WEATHER · {data().weather.place}</p>
          <div>
            <strong aria-label={data().weather.condition}>
              {/* @client */ weatherGlyph(data().weather.code)}
            </strong>
            <p className="temperature">
              {data().weather.temperature.toFixed(1)}
              <small>°C</small>
            </p>
          </div>
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
        <div className="hourly-weather">
          <header>
            <p className="eyebrow">HOURLY FORECAST · NEXT 8 HOURS</p>
            <span>降水確率 / 降水量 / 風速</span>
          </header>
          <div className="hourly-list">
            {data().hourly.map((hour) => (
              <div className="hourly-item" key={hour.time}>
                <time>{hour.time}</time>
                <strong aria-label={hour.condition}>{/* @client */ weatherGlyph(hour.code)}</strong>
                <b>{hour.temperature.toFixed(0)}°</b>
                <p>
                  <span>RAIN</span> {hour.rain}%
                </p>
                <p>
                  <span>PRECIP</span> {hour.precip.toFixed(1)} mm
                </p>
                <p>
                  <span>WIND</span> {hour.wind.toFixed(0)} km/h
                </p>
              </div>
            ))}
          </div>
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
          {data().events.length ? (
            <ol>
              {data().events.map((event) => (
                <li className={"schedule-" + event.day} key={event.id}>
                  <time>
                    <small>{event.day === "today" ? "今日" : "明日"}</small>
                    {eventTime(event)}
                  </time>
                  <i />
                  <div>
                    <strong>{event.title}</strong>
                    {event.location && <small>{event.location}</small>}
                  </div>
                </li>
              ))}
            </ol>
          ) : (
            <p className="empty">直近の予定はありません</p>
          )}
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
          <div className="attendance-data" hidden={!data().attendance.available}>
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
          </div>
          <div className="empty attendance-empty" hidden={data().attendance.available}>
            FLEX_TIMER_URL を設定すると表示されます
          </div>
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
          <div className="news-takeover-title">
            <p>
              {
                /* @client */ now().toLocaleTimeString("ja-JP", {
                  hour: "2-digit",
                  minute: "2-digit",
                })
              }
            </p>
            <h2>ニュース</h2>
          </div>
          <ol>
            {data()
              .news.slice(newsPageStart(), newsPageStart() + 3)
              .map((item, index) => (
                <li key={item.url}>
                  <b>{String(newsPageStart() + index + 1).padStart(2, "0")}</b>
                  <div>
                    <time>{item.published || "--:--"}</time>
                    <strong>{item.title}</strong>
                    <p>{item.summary}</p>
                  </div>
                </li>
              ))}
          </ol>
          <footer>
            <span>
              {String(newsPageStart() / 3 + 1).padStart(2, "0")} /{" "}
              {String(Math.ceil(data().news.length / 3)).padStart(2, "0")} · 更新{" "}
              {data().generatedAt.slice(11, 16)}
            </span>
          </footer>
        </section>
      )}

      {scheduleTakeover() && data().events.length > 0 && (
        <section className="news-takeover schedule-takeover" role="status" aria-label="予定">
          <div className="news-takeover-title">
            <p>
              {
                /* @client */ now().toLocaleTimeString("ja-JP", {
                  hour: "2-digit",
                  minute: "2-digit",
                })
              }
            </p>
            <h2>予定</h2>
          </div>
          <ol>
            {data()
              .events.slice(schedulePageStart(), schedulePageStart() + 4)
              .map((event) => (
                <li key={event.id}>
                  <b>{event.day === "today" ? "今日" : "明日"}</b>
                  <div>
                    <time>{eventTime(event)}</time>
                    <strong>{event.title}</strong>
                    <p>{event.location || "場所の指定なし"}</p>
                  </div>
                </li>
              ))}
          </ol>
          <footer>
            <span>
              {String(schedulePageStart() / 4 + 1).padStart(2, "0")} /{" "}
              {String(Math.ceil(data().events.length / 4)).padStart(2, "0")} · 更新{" "}
              {data().generatedAt.slice(11, 16)}
            </span>
          </footer>
        </section>
      )}

      <section
        className={`news-takeover attendance-takeover ${attendanceTakeover() && data().attendance.available ? "is-visible" : ""}`}
        role="status"
        aria-label="勤怠"
        aria-hidden={!attendanceTakeover() || !data().attendance.available}
      >
        <div className="news-takeover-title">
          <p>
            {
              /* @client */ now().toLocaleTimeString("ja-JP", {
                hour: "2-digit",
                minute: "2-digit",
              })
            }
          </p>
          <h2>勤怠</h2>
        </div>
        <div className="attendance-takeover-content">
          <div className="attendance-takeover-worked">
            <p>WORKED TODAY</p>
            <strong>{/* @client */ duration(shownWorked())}</strong>
            <span>/ {/* @client */ duration(data().attendance.targetSeconds)}</span>
          </div>
          <dl>
            <div>
              <dt>STATUS</dt>
              <dd>{/* @client */ stateLabel()}</dd>
            </div>
            <div>
              <dt>DAY BALANCE</dt>
              <dd>
                {/* @client */ duration(shownWorked() - data().attendance.targetSeconds, true)}
              </dd>
            </div>
            <div>
              <dt>MONTH BALANCE</dt>
              <dd>{/* @client */ duration(data().attendance.monthDifference, true)}</dd>
            </div>
            <div>
              <dt>MONTH PROJECTION</dt>
              <dd>{/* @client */ duration(data().attendance.projectedSeconds)}</dd>
            </div>
          </dl>
        </div>
        <footer>
          <span>LIVE · 1秒ごとに更新</span>
        </footer>
      </section>

      {eventAlert() && (
        <section
          className="event-alert"
          role="alert"
          aria-live="assertive"
          aria-label="予定時刻です"
        >
          <div className="event-alert-rays" aria-hidden="true" />
          <p className="event-alert-kicker">SCHEDULE IMPACT</p>
          <p className="event-alert-time">{eventAlert()!.time}</p>
          <h2>{eventAlert()!.title}</h2>
          <p className="event-alert-location">{eventAlert()!.location || "予定の時刻です"}</p>
          <strong>予定時刻</strong>
        </section>
      )}

      <BurnInGuard />
    </main>
  );
}
