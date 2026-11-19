import json
import sys
import time

from faster_whisper import WhisperModel


def main() -> None:
    wav, model_name, beam = sys.argv[1], sys.argv[2], int(sys.argv[3])

    t0 = time.monotonic()
    model = WhisperModel(model_name, device="cpu", compute_type="int8", cpu_threads=4)
    load_s = time.monotonic() - t0

    t1 = time.monotonic()
    segments, info = model.transcribe(wav, language="ru", beam_size=beam, vad_filter=True)
    segments = list(segments)
    run_s = time.monotonic() - t1

    rtf = run_s / info.duration
    result = {
        "model": model_name,
        "beam_size": beam,
        "audio_s": round(info.duration, 1),
        "load_s": round(load_s, 1),
        "run_s": round(run_s, 1),
        "rtf": round(rtf, 3),
        "est_4h_h": round(rtf * 4, 2),
        "est_6h_h": round(rtf * 6, 2),
        "segments": len(segments),
    }
    print(json.dumps(result, ensure_ascii=False))
    for s in segments[:40]:
        print(f"[{s.start:7.1f}] {s.text.strip()}")


if __name__ == "__main__":
    main()
