"use client";

import { batch, createMemo, createSignal, onCleanup, onMount } from "@barefootjs/client";

export function createSignageState() {
  // TODO: Move these types to a shared module when https://github.com/piconic-ai/barefootjs/issues/2992 is fixed.
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

  const [data, setData] = createSignal<Dashboard>({
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
  });
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
      const body: Dashboard = await response.json();
      batch(() => {
        setAttendanceUpdatedAt(Date.parse(body.generatedAt));
        setData(body);
      });
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
      batch(() => {
        setAttendanceUpdatedAt(Date.now());
        setData({ ...data(), attendance: body });
      });
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
      batch(() => {
        setNewsTakeover(false);
        setScheduleTakeover(false);
        setAttendanceTakeover(false);
        setEventAlert(event);
      });
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

  const stateLabel = createMemo(
    () =>
      ({ working: "勤務中", break: "休憩中", off: "退勤済" })[data().attendance.state] ?? "未接続",
  );
  const shownWorked = createMemo(
    () =>
      data().attendance.workedSeconds +
      (data().attendance.state === "working"
        ? Math.max(0, (now().getTime() - attendanceUpdatedAt()) / 1000)
        : 0),
  );
  const newsPageStart = createMemo(() => {
    const pages = Math.ceil(data().news.length / 3);
    return pages ? (newsPage() % pages) * 3 : 0;
  });
  const schedulePageStart = createMemo(() => {
    const pages = Math.ceil(data().events.length / 4);
    return pages ? (schedulePage() % pages) * 4 : 0;
  });

  return {
    data,
    now,
    busy,
    newsTakeover,
    scheduleTakeover,
    attendanceTakeover,
    eventAlert,
    soundReady,
    stateLabel,
    shownWorked,
    newsPageStart,
    schedulePageStart,
    enableSound,
    action,
  };
}
