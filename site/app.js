"use strict";

const TYPES = {
  game: "Игра",
  movie: "Фильм",
  series: "Сериал",
  anime: "Аниме",
  cartoon: "Мультфильм",
  book: "Книга",
  comic: "Комикс",
  other: "Другое",
  article: "Статья",
};

const HOSTS = {
  dima: "Дима",
  maxim: "Максим",
  timur: "Тимур",
  team: "Редакция",
  unknown: "Неизвестно",
};

const VERDICTS = {
  positive: "Нравится",
  mixed: "Смешанно",
  negative: "Не понравилось",
};

const ALL = "";

function hostLabel(host) {
  if (host.startsWith("guest:")) {
    return host.slice("guest:".length) + " (гость)";
  }
  return HOSTS[host] || host;
}

function clock(sec) {
  const s = Math.max(0, Math.floor(sec));
  const pad = (n) => String(n).padStart(2, "0");
  return pad(Math.floor(s / 3600)) + ":" + pad(Math.floor((s % 3600) / 60)) + ":" + pad(s % 60);
}

function el(tag, className, text) {
  const node = document.createElement(tag);
  if (className) {
    node.className = className;
  }
  if (text !== undefined) {
    node.textContent = text;
  }
  return node;
}

function link(href, text, external) {
  const a = el("a", "", text);
  a.href = href;
  if (external) {
    a.target = "_blank";
    a.rel = "noopener";
  }
  return a;
}

function fillSelect(select, options, selected) {
  select.replaceChildren();
  for (const [value, label] of options) {
    const opt = el("option", "", label);
    opt.value = value;
    select.append(opt);
  }
  select.value = selected;
}

function card(row) {
  const li = el("li", "card");
  const head = el("div", "card-head");
  head.append(el("h2", "title", row.title));
  if (row.title_ru) {
    head.append(el("span", "title-ru", row.title_ru));
  }
  li.append(head);

  const meta = el("div", "meta");
  meta.append(el("span", "tag", TYPES[row.type] || row.type));
  meta.append(el("span", "tag", hostLabel(row.host)));
  meta.append(el("span", "tag verdict-" + row.verdict, VERDICTS[row.verdict] || row.verdict));
  li.append(meta);

  if (row.summary) {
    li.append(el("p", "summary", row.summary));
  }

  const links = el("div", "links");
  if (row.page_url) {
    links.append(link(row.page_url, "Выпуск " + row.episode, true));
  }
  if (row.type === "article") {
    if (row.url) {
      links.append(link(row.url, "Открыть", true));
    }
  } else if (row.audio_url) {
    links.append(link(row.audio_url + "#t=" + row.t, "▶ " + clock(row.t), true));
  }
  li.append(links);
  return li;
}

function setup(rows) {
  const q = document.getElementById("q");
  const type = document.getElementById("type");
  const host = document.getElementById("host");
  const verdict = document.getElementById("verdict");
  const episode = document.getElementById("episode");
  const count = document.getElementById("count");
  const list = document.getElementById("list");

  fillSelect(type, [[ALL, "Все"], ...Object.entries(TYPES)], ALL);

  const hosts = [...new Set(rows.map((r) => r.host))].sort((a, b) =>
    hostLabel(a).localeCompare(hostLabel(b), "ru"),
  );
  fillSelect(host, [[ALL, "Все"], ...hosts.map((h) => [h, hostLabel(h)])], ALL);

  fillSelect(verdict, [[ALL, "Все"], ...Object.entries(VERDICTS)], "positive");

  const episodes = new Map();
  for (const r of rows) {
    episodes.set(r.episode, r.episode_title || "Выпуск " + r.episode);
  }
  const epOptions = [...episodes.entries()]
    .sort((a, b) => b[0] - a[0])
    .map(([n, title]) => [String(n), title]);
  fillSelect(episode, [[ALL, "Все"], ...epOptions], ALL);

  function render() {
    const needle = q.value.trim().toLowerCase();
    const shown = rows.filter((r) => {
      if (type.value && r.type !== type.value) return false;
      if (host.value && r.host !== host.value) return false;
      if (verdict.value && r.verdict !== verdict.value) return false;
      if (episode.value && String(r.episode) !== episode.value) return false;
      if (needle) {
        const hay = (r.title + " " + (r.title_ru || "")).toLowerCase();
        if (!hay.includes(needle)) return false;
      }
      return true;
    });
    count.textContent = "Найдено: " + shown.length;
    list.replaceChildren(...shown.map(card));
  }

  document.getElementById("controls").addEventListener("submit", (e) => e.preventDefault());
  q.addEventListener("input", render);
  for (const s of [type, host, verdict, episode]) {
    s.addEventListener("change", render);
  }
  render();
}

async function main() {
  const count = document.getElementById("count");
  try {
    const res = await fetch("data.json");
    if (!res.ok) {
      throw new Error("http " + res.status);
    }
    const rows = await res.json();
    if (!Array.isArray(rows)) {
      throw new Error("bad data");
    }
    setup(rows);
  } catch (err) {
    count.textContent = "Не удалось загрузить данные. Попробуйте обновить страницу.";
    count.classList.add("error");
    console.error(err);
  }
}

main();
