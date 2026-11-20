# zavtracast-recs

Поиск по рекомендациям ведущих подкаста «Завтракаст»: игры, фильмы, сериалы,
книги и всё остальное, о чём ведущие рассказывали в выпусках, с вердиктом,
кратким пересказом и ссылкой на момент в аудио.

Сайт: https://fee1good.github.io/zavtracast-recs/

## Как это работает

Раз в день GitHub Actions читает RSS-ленту подкаста, находит новые выпуски,
транскрибирует аудио через faster-whisper, отдаёт транскрипт и шоуноты в
Gemini, который выделяет рекомендации ведущих, и сохраняет результат в
`data/episodes/<N>.json`. Транскрипты хранятся в отдельном приватном
репозитории `fee1good/zavtracast-recs-transcripts`. После этого из всех
файлов выпусков собирается статический сайт и публикуется в GitHub Pages.

Джобы workflow `update`:

- `discover`: ищет в ленте выпуски без файла данных и отдаёт матрицу из
  тех, у кого ещё нет транскрипта;
- `transcribe`: отдельная джоба на каждый выпуск, скачивает mp3 и
  транскрибирует его;
- `classify`: сохраняет транскрипты, извлекает рекомендации и коммитит
  `data/episodes`;
- `publish`: собирает сайт и деплоит его в GitHub Pages.

## Локальный запуск

```sh
go test ./...
go run ./cmd/recs discover -feed internal/feed/testdata/feed.xml
go run ./cmd/recs build -out _site
go run ./cmd/recs eval -golden eval/golden
```

`classify` требует переменную окружения `GEMINI_API_KEY` и транскрипты в
`.transcripts/`.

Тесты транскрибатора:

```sh
uv run --python 3.12 --with-requirements transcribe/requirements.txt \
  python -m unittest discover -s transcribe -v
```

## Секреты и переменные

Секреты репозитория:

| Имя | Зачем |
| --- | --- |
| `GEMINI_API_KEY` | ключ Gemini API для извлечения рекомендаций |
| `TRANSCRIPTS_TOKEN` | fine-grained PAT с правом Contents read/write на `zavtracast-recs-transcripts` |

Переменные репозитория:

| Имя | Зачем |
| --- | --- |
| `GOATCOUNTER_CODE` | код GoatCounter для счётчика посещений |
| `RECS_GEMINI_MODEL` | необязательно, модель Gemini вместо модели по умолчанию |

## Перезапуск одного выпуска

Actions → `update` → Run workflow, в поле `episode` указать номер выпуска.
Если транскрипта ещё нет, выпуск сначала транскрибируется. Рекомендации
извлекаются повторно, файл `data/episodes/<N>.json` перезаписывается.

## Дисклеймер

Неофициальный фанатский проект, не связан с авторами подкаста
(https://zavtracast.ru). Записи извлечены автоматически и могут содержать
ошибки.
