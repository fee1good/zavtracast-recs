import argparse
import json
import os
import sys
import time

from faster_whisper import WhisperModel, decode_audio

SAMPLE_RATE = 16000
PROGRESS_EVERY = 600.0


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    p = argparse.ArgumentParser(description="transcribe an episode with faster-whisper")
    p.add_argument("--audio", required=True)
    p.add_argument("--episode", type=int, required=True)
    p.add_argument("--out", required=True)
    p.add_argument("--model", default="large-v3-turbo")
    p.add_argument("--beam", type=int, default=5)
    p.add_argument("--threads", type=int, default=4)
    p.add_argument("--start", type=float, default=0.0)
    return p.parse_args(argv)


def transcribe(args: argparse.Namespace) -> dict:
    audio = decode_audio(args.audio, sampling_rate=SAMPLE_RATE)
    duration = len(audio) / SAMPLE_RATE
    start = max(args.start, 0.0)
    audio = audio[int(start * SAMPLE_RATE):]

    model = WhisperModel(
        args.model, device="cpu", compute_type="int8", cpu_threads=args.threads
    )
    segments, _ = model.transcribe(
        audio, language="ru", beam_size=args.beam, vad_filter=True
    )

    t0 = time.monotonic()
    next_mark = start + PROGRESS_EVERY
    out = []
    for s in segments:
        seg_start = s.start + start
        seg_end = s.end + start
        out.append(
            {
                "start": round(seg_start, 2),
                "end": round(seg_end, 2),
                "text": s.text.strip(),
            }
        )
        if seg_end >= next_mark:
            elapsed = time.monotonic() - t0
            print(
                f"progress {seg_end:.0f}/{duration:.0f} s, elapsed {elapsed:.0f} s",
                file=sys.stderr,
                flush=True,
            )
            next_mark = (seg_end // PROGRESS_EVERY + 1) * PROGRESS_EVERY

    return {
        "episode": args.episode,
        "model": args.model,
        "duration": round(duration, 2),
        "segments": out,
    }


def write_json(path: str, data: dict) -> None:
    tmp = path + ".tmp"
    with open(tmp, "w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False)
        f.write("\n")
    os.replace(tmp, path)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    write_json(args.out, transcribe(args))
    return 0


if __name__ == "__main__":
    sys.exit(main())
