# zavtracast-recs pipeline

Goal: a daily GitHub Actions pipeline that transcribes new Zavtracast
episodes with faster-whisper, extracts hosts' recommendations with Gemini,
stores them as `data/episodes/<N>.json` and deploys a static search site to
GitHub Pages.

Spec: `docs/specs/2026-11-19-design.md`. Read it before starting any task.
This plan wins where the two differ.

## Validation Commands

- `go vet ./...`
- `test -z "$(gofmt -l .)"`
- `go test -race ./...`
- `uv run --python 3.12 --with-requirements transcribe/requirements.txt python -m unittest discover -s transcribe -v`
- `node --check site/app.js`
- `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`

## Global Constraints

- Go module `github.com/fee1good/zavtracast-recs`, `go 1.26` in go.mod.
  Use modern Go: `new(value)`, `strings.SplitSeq`, `cmp.Or`, `slices`,
  `maps`, `errors.Join`, `t.Context()`, `omitzero`, range over int,
  `for b.Loop()`. Lines up to 100 chars.
- Only dependency outside stdlib: `google.golang.org/genai v1.79.1`.
- No comments in code of any kind (no doc comments, no inline, no TODO).
- Log and error messages lowercase and short, `log/slog` for logging.
- Do not mention any AI assistant or vendor tooling in code, commits or
  files. Commit messages: one line, `<type>(<scope>): <description>`.
- Python 3.12, dependencies pinned exactly in `transcribe/requirements.txt`.
- GitHub Actions versions: `actions/checkout@v7`, `actions/setup-python@v7`,
  `actions/setup-go@v7`, `actions/upload-artifact@v7`,
  `actions/download-artifact@v8`, `actions/cache@v6`,
  `actions/upload-pages-artifact@v5`, `actions/deploy-pages@v5`.
- Workflow expressions from inputs, matrix or vars never go straight into
  `run:`; pass them through `env:` and use `"$VAR"`.
- Tests never hit the network, except the Python smoke test that downloads
  the `tiny` whisper model.

### Task 1: Scaffold module, clock helpers, drop the spike

- [x] `go mod init github.com/fee1good/zavtracast-recs`, set `go 1.26`
- [x] Delete `spike/` and `.github/workflows/spike.yml`
- [x] `.gitignore`: `.transcripts/`, `_site/`, `*.mp3`, `__pycache__/`, `.venv/`
- [x] `internal/clock`: `Format(sec int) string` returns `%02d:%02d:%02d`
      (hours may exceed 99 without truncation); `Parse(s string) (int, error)`
      accepts `H:MM:SS`, `MM:SS` and `SS` via `strings.SplitSeq`, rejects
      empty parts, negatives, minutes or seconds >= 60 when a higher part is
      present
- [x] Table tests for both: `0`, `59`, `3600`, `8261` (`02:17:41`),
      `"4:11:14"` = 15074, `"1:02"` = 62, `"7"` = 7, errors on `""`, `"1::2"`,
      `"a:00"`, `"1:60:00"`, round trip `Parse(Format(x)) == x`
- [x] Validation commands pass (Go ones; others may not exist yet)

### Task 2: RSS feed parsing

Fixture `internal/feed/testdata/feed.xml` is a trimmed real feed with
items «Завтракаст 395», «Завтракаст 394» and «Магнитное поле №14».

- [x] `internal/feed`: `const URL = "https://feeds.podcastmirror.com/zavtracast"`
- [x] Types:
      `Article{Title, URL string}`;
      `Episode{Number int; Title string; Date time.Time; PageURL, AudioURL string;
      Duration int; Shownotes, Impressions []string; Articles []Article}`
- [x] `Parse(r io.Reader) ([]Episode, error)` with `encoding/xml`:
      `title`, `link` (PageURL), `pubDate` (RFC1123Z),
      `enclosure` `url,attr` (AudioURL),
      `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd duration"` parsed with
      `clock.Parse`,
      `xml:"http://purl.org/rss/1.0/modules/content/ encoded"` as HTML source.
      Keep only items whose title matches `^Завтракаст (\d+)\b`; Number from
      the match. Result sorted by Number ascending
- [x] Shownotes: in `content:encoded` find the heading containing «Шоуноты»,
      take the first `<ol>` after it up to its `</ol>`, extract `<li>` items
      with `(?s)<li[^>]*>(.*?)</li>`; text = tags stripped,
      `html.UnescapeString`, whitespace collapsed, trimmed; skip empty
- [x] Impressions: shownote items matching `^(Дима|Максим|Макс|Тимур)[\s,]`
- [x] Articles: heading containing «материалы недели» (both «Самые интересные
      материалы недели» and «Лучшие материалы недели»), first `<ol>` after it;
      per `<li>`: URL = first `href` value, HTML-unescaped; Title = li text
      with tags and bare `http(s)://` URLs removed, trimmed, trailing `:` `-`
      `–` stripped; skip items without URL or title
- [x] `Fetch(ctx context.Context, url string) ([]Episode, error)`: GET with
      60 s timeout, non-2xx is an error, then `Parse`
- [x] Tests on the fixture:
      2 episodes (394, 395), Магнитное поле excluded;
      395: Duration 8261, PageURL `https://zavtracast.ru/395.html`,
      Date `2026-11-11 17:25:38 +0000`, 8 shownotes, 3 impressions, the last
      starts with «Дима и Максим поиграли в The Legend of Zelda: Ocarina of Time»,
      0 articles, AudioURL non-empty with `.mp3`;
      394: Duration 15074, 15 shownotes, 8 impressions, first is
      «Дима про Steam Controller.», 1 article
      `{«Как американцы нашли русский город на Аляске»,
      "https://youtu.be/TMQ31DQjjWg?is=KMn2Gib7v4dC4wpH"}`
- [x] `Fetch` test with `httptest.Server` serving the fixture and one
      returning 500

### Task 3: Transcript files

- [x] `internal/transcript`:
      `Segment{Start, End float64; Text string}` (json `start`,`end`,`text`),
      `Transcript{Episode int; Model string; Duration float64; Segments []Segment}`
      (json `episode`,`model`,`duration`,`segments`) matching the output of
      `transcribe.py`
- [x] `Path(dir string, n int) string` = `<dir>/<n>.json`;
      `Exists(dir, n) bool`; `Load(dir, n) (Transcript, error)`
- [x] `(Transcript) Lines() string`: one line per segment
      `[HH:MM:SS] text\n` using `clock.Format(int(Start))`, text trimmed,
      empty texts skipped
- [x] Tests: load from `t.TempDir()`, `Exists` true/false, `Lines` output
      for 3 segments incl. one empty

### Task 4: Records and episode files

- [x] `internal/record`:
      `Record{ID, Title, TitleRU, Type, Host, Verdict, Summary, URL string; T int}`
      json: `id`, `title`, `title_ru,omitzero`, `type`, `host`, `verdict`,
      `summary,omitzero`, `url,omitzero`, `t`
- [x] `MediaTypes = []string{"game","movie","series","anime","cartoon","book","comic","other"}`,
      `const TypeArticle = "article"`, `Verdicts = []string{"positive","mixed","negative"}`,
      `Hosts = []string{"dima","maxim","timur","unknown"}`, `const HostTeam = "team"`
- [x] `ValidHost(h string) bool`: one of Hosts, or `guest:<name>` with
      non-blank name
- [x] `(Record) Validate(duration int) error`: type in MediaTypes or article;
      verdict in Verdicts; non-article: host valid, summary non-blank, url
      empty; article: host `team`, url starts with `http`; title non-blank;
      `0 <= T <= duration`
- [x] `Slug(s string) string`: lowercase, unicode letters and digits kept,
      everything else becomes `-`, runs collapsed, trimmed of `-`, cut to
      80 runes and re-trimmed. `MakeID(ep int, title, host string)` =
      `<ep>-<Slug(title)>-<Slug(host)>`
- [x] `Clean(ep, duration int, recs []Record) (kept []Record, dropped []error)`:
      sets ID via MakeID, drops invalid and duplicate IDs (first wins), each
      drop is an error naming the title and reason; order preserved
- [x] `var ErrTooManyInvalid`; `CheckDropped(total, dropped int) error`
      returns it when `dropped*3 > total`
- [x] Episode file: `Episode{Episode int; Title, Date, PageURL, AudioURL string;
      Duration int; Records []Record}` json `episode`,`title`,`date`
      (`2006-01-02`), `page_url`, `audio_url`, `duration`, `records`;
      `Path(dir, n)`, `Exists(dir, n)`, `Write(dir string, e Episode) error`
      (MkdirAll, 2-space indent, trailing newline, `records` is `[]` never
      `null`, `SetEscapeHTML(false)`), `Read(dir, n)`,
      `ReadAll(dir) ([]Episode, error)` sorted by episode ascending, missing
      dir = empty result
- [x] Tests: Slug on `"The Legend of Zelda: Ocarina of Time"` →
      `the-legend-of-zelda-ocarina-of-time`, Cyrillic `"Пикник на обочине"` →
      `пикник-на-обочине`, `"guest:Иван"` → `guest-иван`, long input ≤ 80
      runes; Validate table for every rule; Clean with duplicate and invalid;
      CheckDropped boundaries (3 of 9 ok, 4 of 9 error, 0 of 0 ok);
      Write/Read round trip and golden bytes for a small episode

### Task 5: Classifier

- [x] `internal/classify`: `type Generator interface { Generate(ctx context.Context,
      system, user string) (string, error) }`
- [x] `SystemPrompt` (Russian, const) with the rules: a record is a host's
      own experience with a work (played, watched, read) with an opinion;
      skip news, rumors, announcements, trailers, retellings of others'
      reviews; one record per work and host, two hosts with different opinions
      give two records; host is `dima`, `maxim`, `timur`, `guest:<имя>` or
      `unknown` when unsure; title in the original language as officially
      written, `title_ru` only if a Russian title exists and differs, else
      null; summary paraphrased in Russian, 1-2 sentences, no quotes; `time` is
      the `[HH:MM:SS]` of the line where the discussion of the work starts;
      the impressions checklist from shownotes: find each item in the
      transcript and use it for spelling of titles and hosts; an item with no
      trace in the transcript produces no record; transcript is the source of
      truth, it has no punctuation and may misspell foreign names
- [x] `UserPrompt(ep feed.Episode, tr transcript.Transcript) string`:
      sections «Выпуск» (title), «Впечатления из шоунотов» (numbered
      Impressions), «Шоуноты» (numbered Shownotes), «Транскрипт» (`tr.Lines()`)
- [x] Reply type `{records: [{title, title_ru, type, host, verdict, summary,
      time}]}`; `Schema() *genai.Schema` with enums for type (MediaTypes) and
      verdict, `title_ru` nullable, all fields required
- [x] `Classifier{Gen Generator; Backoff []time.Duration}`;
      `Classify(ctx, ep, tr) ([]record.Record, error)`: calls Gen, parses JSON;
      on Gen error or invalid JSON retries after each Backoff entry (stops on
      ctx done), returns the last error wrapped after the last attempt.
      `time` → T via `clock.Parse`, unparsable → T = -1 so validation drops
      it. `DefaultBackoff = []time.Duration{10*time.Second, 30*time.Second, 90*time.Second}`
- [x] `Articles(ep feed.Episode) []record.Record`: type article, host team,
      verdict positive, T 0, Title and URL from the article
- [x] `Gemini` adapter in `gemini.go`, following the shape of
      `NewGemini/Complete` from the rentradar project: `NewGemini(ctx,
      apiKey, model string) (*Gemini, error)` with `genai.BackendGeminiAPI`;
      `Generate` with 10 min timeout, `ResponseMIMEType "application/json"`,
      `ResponseSchema: Schema()`, `Temperature: new(float32(0))`,
      `MaxOutputTokens: 32768`, system as `SystemInstruction`; empty text is
      an error that includes finish reason and block reason; logs
      `slog.Info("gemini usage", "model", ..., "in", ..., "out", ...)`.
      `const DefaultModel = "gemini-3.6-flash"`
- [x] Tests with a fake Generator: happy path maps all fields and T;
      `title_ru` null → empty; invalid time → T -1; error twice then success
      with zero backoff → 3 calls; always error → error after
      `len(Backoff)+1` calls; invalid JSON retried; cancelled ctx stops
      retries; UserPrompt contains impressions, shownotes and a transcript
      line; Articles mapping. No test calls the real API

### Task 6: Site data build and eval

- [x] `internal/site`: `Row` embeds `record.Record` plus `Episode int`,
      `EpisodeTitle`, `Date`, `PageURL`, `AudioURL` (json `episode`,
      `episode_title`, `date`, `page_url`, `audio_url`)
- [x] `Rows(eps []record.Episode) []Row` sorted by episode desc, then T asc,
      then ID
- [x] `Build(siteDir, outDir string, eps []record.Episode, goatcounter string) error`:
      MkdirAll outDir; copy `index.html`, `app.js`, `style.css`; in index.html
      replace the marker `<!--goatcounter-->` with
      `<script data-goatcounter="https://<code>.goatcounter.com/count" async src="//gc.zgo.at/count.js"></script>`
      when goatcounter is non-empty, else remove the marker; code must match
      `^[a-z0-9-]+$` or Build fails; write `data.json` as a JSON array of
      Row (`[]` when empty), no HTML escaping
- [x] `internal/eval`: `Result{Found, Missed, Extra, VerdictMismatch []string}`;
      `Compare(golden, got []record.Record) Result` keyed by
      `Slug(Title)+"|"+Host`; VerdictMismatch lists keys found in both with
      different verdicts; article records ignored on both sides;
      `(Result) Precision() float64`, `Recall() float64` (0 when undefined)
- [x] Tests: Rows ordering across 2 episodes; Build with temp site dir checks
      copied files, marker replaced / removed, bad code rejected, data.json
      content; Compare with found, missed, extra, mismatch and articles ignored

### Task 7: `recs` CLI

- [x] `cmd/recs/main.go`: subcommands `discover`, `classify`, `build`,
      `eval` via `flag.NewFlagSet`; shared flags
      `-feed` (default `feed.URL`, a value without `http` prefix is read as a
      local file), `-data data/episodes`, `-transcripts .transcripts`,
      `-min 386`, `-episode 0`; exit code 1 on error with `slog.Error`
- [x] `discover`: fetch feed; candidates = episodes with Number >= min and
      no data file; with `-episode N` the candidate set is just N (must exist
      in the feed, else error) regardless of data file. Matrix = candidates
      without a transcript. Prints to stdout exactly two lines:
      `matrix=<compact JSON array of {"episode":N,"audio_url":"..."}>` and
      `count=<len>`. Empty matrix prints `matrix=[]`
- [x] `classify`: flags `-model` (default `cmp.Or(os.Getenv("RECS_GEMINI_MODEL"),
      classify.DefaultModel)`); requires `GEMINI_API_KEY`. Targets = feed
      episodes >= min with a transcript and no data file, or just `-episode N`
      when set (overwrites). Per target: Classify, `record.Clean` LLM records,
      `CheckDropped` (on error skip writing this episode), Clean articles
      separately and only log their drops, write episode file with LLM records
      then articles. Logs each dropped record. Continues to the next episode on
      error; returns `errors.Join` of all episode errors
- [x] `build`: flags `-site site`, `-out _site`, `-goatcounter`; ReadAll data
      → `site.Build`
- [x] `eval`: flag `-golden eval/golden`; for each golden episode file with a
      matching data file print `episode N: found X missed Y extra Z verdict W`
      and the missed / extra keys, then totals with precision and recall
- [x] Tests for discover and classify logic with injected deps: factor the
      bodies into functions taking `[]feed.Episode`, dirs and a
      `classify.Generator`, test with the fixture feed, temp dirs and a fake
      generator (state 1 / 2 / 3 episodes, forced episode, unknown forced
      episode, one episode failing does not stop the next, threshold skip)
- [x] `data/episodes/.gitkeep`, `eval/golden/.gitkeep`

### Task 8: Transcriber

- [x] `transcribe/requirements.txt` with exactly:
      `anyio==4.16.0 av==18.1.0 click==8.5.1 ctranslate2==4.9.0
      faster-whisper==1.2.1 filelock==4.2.12 flatbuffers==25.12.19
      fsspec==2026.9.0 h11==0.16.0 hf-xet==1.8.0 httpcore2==2.16.2
      httpx2==2.16.2 huggingface-hub==2.4.0 idna==3.20 numpy==2.5.5
      onnxruntime==1.32.0 packaging==26.3 protobuf==7.36.2 pyyaml==6.0.3
      tokenizers==0.23.3 tqdm==4.70.1 truststore==0.10.5
      typing-extensions==4.16.0` (one per line)
- [x] `transcribe/transcribe.py`, argparse: `--audio` (required),
      `--episode` (int, required), `--out` (required), `--model
      large-v3-turbo`, `--beam 5`, `--threads 4`, `--start 0.0`
- [x] Load audio with `faster_whisper.decode_audio(path, sampling_rate=16000)`,
      duration = `len(audio)/16000`, slice off the first `start` seconds,
      transcribe the array with `WhisperModel(model, device="cpu",
      compute_type="int8", cpu_threads=threads)` and `language="ru",
      beam_size=beam, vad_filter=True`; add `start` back to segment times
- [x] Progress to stderr every 10 minutes of audio processed:
      `progress <pos>/<duration> s, elapsed <s> s`
- [x] Write `{episode, model, duration, segments:[{start,end,text}]}`
      (times rounded to 2 decimals, text stripped, `ensure_ascii=False`) to
      `<out>.tmp`, then `os.replace` to `out`
- [x] `transcribe/test_transcribe.py` (unittest): build a 3 s silent 16 kHz
      mono wav with the `wave` module in a temp dir, run the script via
      `subprocess` with `sys.executable`, `--model tiny --beam 1`; assert
      exit 0, JSON keys, episode, model `tiny`, duration ≈ 3.0, segments is a
      list; second test with `--start 1` keeps duration ≈ 3.0; third test
      missing `--audio` exits non-zero

### Task 9: Static site

- [x] `site/index.html`: `lang="ru"`, title «Рекомендации Завтракаста»,
      viewport meta, links `style.css`, `app.js` (defer), `<!--goatcounter-->`
      marker in head; header with title and one-line description; controls:
      search input, selects for type, host, verdict (default «positive»),
      episode; result count; list container; footer «Неофициальный фанатский
      проект, не связан с авторами подкаста. Записи извлечены автоматически и
      могут содержать ошибки.» with link to
      `https://github.com/fee1good/zavtracast-recs` and to `https://zavtracast.ru`
- [x] `site/app.js` (vanilla, no deps): fetch `data.json`; fill episode and
      host selects from data; labels: types (game Игра, movie Фильм, series
      Сериал, anime Аниме, cartoon Мультфильм, book Книга, comic Комикс, other
      Другое, article Статья), hosts (dima Дима, maxim Максим, timur Тимур,
      team Редакция, unknown Неизвестно, `guest:X` → `X (гость)`), verdicts
      (positive Нравится, mixed Смешанно, negative Не понравилось, plus «Все»);
      search is case-insensitive over title and title_ru; card shows title,
      title_ru, type, host, verdict, summary, link «Выпуск N» to page_url and
      for non-articles `▶ HH:MM:SS` to `audio_url#t=<t>`, for articles
      «Открыть» to url (`rel="noopener"`, `target="_blank"`); build DOM with
      `textContent` only, never `innerHTML` with data; error message when
      data.json fails to load
- [x] `site/style.css`: system font stack, max width ~900px, card list,
      readable on 360px width without horizontal scroll, dark theme via
      `prefers-color-scheme`
- [x] Check: `go run ./cmd/recs build -out _site` with a hand-written sample
      episode in a temp data dir, open `_site/index.html` via
      `python3 -m http.server` and verify with a headless fetch that
      `data.json` is served; `node --check site/app.js` passes

### Task 10: Workflows

- [x] `.github/workflows/ci.yml`: on `push` and `pull_request`; job `go`:
      checkout, setup-go (`go-version-file: go.mod`), `go vet ./...`,
      gofmt check, `go test -race ./...`; job `python`: checkout,
      setup-python 3.12 with pip cache on `transcribe/requirements.txt`,
      `pip install -r transcribe/requirements.txt`,
      `python -m unittest discover -s transcribe -v`;
      `permissions: contents: read`
- [x] `.github/workflows/update.yml`:
      triggers `schedule: cron "0 6 * * *"`, `workflow_dispatch` with input
      `episode` (string, default empty), `push` on `main` with paths
      `site/**`; `concurrency: {group: update, cancel-in-progress: false}`;
      `env: MIN_EPISODE: "386"`; top-level `permissions: contents: read`
- [x] Job `discover`: checkout; checkout
      `fee1good/zavtracast-recs-transcripts` with `token:
      ${{ secrets.TRANSCRIPTS_TOKEN }}` into `.transcripts`; setup-go;
      step id `d`: `go run ./cmd/recs discover -min "$MIN_EPISODE" -episode
      "$EPISODE" >> "$GITHUB_OUTPUT"` with `EPISODE: ${{ inputs.episode || '0' }}`;
      outputs `matrix`, `count`
- [x] Job `transcribe`: `needs: discover`, `if: needs.discover.outputs.count != '0'`,
      `timeout-minutes: 350`, `strategy: {fail-fast: false, max-parallel: 5,
      matrix: {include: ${{ fromJSON(needs.discover.outputs.matrix) }}}}`;
      steps: checkout, setup-python 3.12 with pip cache, pip install,
      `actions/cache` for `~/.cache/huggingface` keyed
      `hf-large-v3-turbo-${{ hashFiles('transcribe/requirements.txt') }}`,
      `curl -fsSL --retry 3 -o episode.mp3 "$AUDIO_URL"`, run
      `transcribe.py --audio episode.mp3 --episode "$EPISODE" --out "$EPISODE.json"`,
      upload-artifact `transcript-<episode>` with `retention-days: 7`
- [x] Job `classify`: `needs: [discover, transcribe]`,
      `if: always() && needs.discover.result == 'success'`,
      `permissions: contents: write`; steps: checkout `ref: main`; checkout
      transcripts repo into `.transcripts` with the token; download-artifact
      `pattern: transcript-*`, `merge-multiple: true`, `path: .transcripts`,
      `continue-on-error: true`; push new transcripts (git identity
      `github-actions[bot]` / `41898+github-actions[bot]@users.noreply.github.com`,
      commit only if `git status --porcelain` is non-empty, message
      `chore: add transcripts`); setup-go; step id `classify` with
      `continue-on-error: true`, env `GEMINI_API_KEY`, `RECS_GEMINI_MODEL:
      ${{ vars.RECS_GEMINI_MODEL }}`, `EPISODE`, runs `recs classify`; commit
      `data/episodes` if changed with message `chore(data): update episodes`,
      `git pull --rebase` then push; final step
      `if: steps.classify.outcome == 'failure'` runs `exit 1`
- [x] Job `publish`: `needs: [discover, classify]`,
      `if: always() && needs.discover.result == 'success'`, permissions
      `pages: write`, `id-token: write`, `contents: read`, `environment:
      {name: github-pages, url: ${{ steps.deploy.outputs.page_url }}}`;
      checkout `ref: main` (to include the data commit), setup-go,
      `recs build -out _site -goatcounter "$GOATCOUNTER_CODE"` with
      `GOATCOUNTER_CODE: ${{ vars.GOATCOUNTER_CODE }}`, upload-pages-artifact
      `path: _site`, deploy-pages id `deploy`
- [x] actionlint passes on both workflows

### Task 11: README

- [ ] `README.md` in Russian, short: what the site is (link
      `https://fee1good.github.io/zavtracast-recs/`), how the pipeline works
      (one paragraph + the job list), local commands (`go test ./...`,
      `go run ./cmd/recs discover -feed internal/feed/testdata/feed.xml`,
      `build`, `eval`), required secrets and variables (`GEMINI_API_KEY`,
      `TRANSCRIPTS_TOKEN`, `GOATCOUNTER_CODE`, optional `RECS_GEMINI_MODEL`),
      how to rerun one episode (`workflow_dispatch` with `episode`), and the
      unofficial fan project disclaimer

## Manual steps (owner, outside this plan)

1. Create private repo `fee1good/zavtracast-recs-transcripts` with an
   initial commit (empty README) so checkout works.
2. Fine-grained PAT: repository access only `zavtracast-recs-transcripts`,
   Contents read/write → secret `TRANSCRIPTS_TOKEN` in `zavtracast-recs`.
3. Gemini API key from AI Studio → secret `GEMINI_API_KEY`.
4. Settings → Pages → Source: GitHub Actions.
5. GoatCounter account → repo variable `GOATCOUNTER_CODE`.
6. Optional repo variable `RECS_GEMINI_MODEL`.
7. Run `update` via workflow_dispatch, check the site, then review records
   of 386–395 and save corrected copies into `eval/golden/`.
