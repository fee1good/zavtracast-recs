import json
import os
import subprocess
import sys
import tempfile
import unittest
import wave
from types import SimpleNamespace

import transcribe

SCRIPT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "transcribe.py")
RATE = 16000


def write_silence(path: str, seconds: float) -> None:
    with wave.open(path, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(RATE)
        w.writeframes(b"\x00\x00" * int(seconds * RATE))


class TranscribeTest(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.audio = os.path.join(self.tmp.name, "silence.wav")
        self.out = os.path.join(self.tmp.name, "7.json")
        write_silence(self.audio, 3.0)

    def run_script(self, *extra: str) -> subprocess.CompletedProcess:
        return subprocess.run(
            [sys.executable, SCRIPT, *extra],
            check=False,
            capture_output=True,
            text=True,
            timeout=600,
        )

    def transcribe(self, *extra: str) -> dict:
        res = self.run_script(
            "--audio", self.audio, "--episode", "7", "--out", self.out,
            "--model", "tiny", "--beam", "1", *extra,
        )
        self.assertEqual(res.returncode, 0, res.stderr)
        self.assertFalse(os.path.exists(self.out + ".tmp"))
        with open(self.out, encoding="utf-8") as f:
            return json.load(f)

    def test_output_shape(self) -> None:
        data = self.transcribe()
        self.assertEqual(set(data), {"episode", "model", "duration", "segments"})
        self.assertEqual(data["episode"], 7)
        self.assertEqual(data["model"], "tiny")
        self.assertAlmostEqual(data["duration"], 3.0, delta=0.05)
        self.assertIsInstance(data["segments"], list)

    def test_start_keeps_duration(self) -> None:
        data = self.transcribe("--start", "1")
        self.assertAlmostEqual(data["duration"], 3.0, delta=0.05)
        self.assertIsInstance(data["segments"], list)
        for s in data["segments"]:
            self.assertGreaterEqual(s["start"], 1.0)

    def test_missing_audio_arg_fails(self) -> None:
        res = self.run_script("--episode", "7", "--out", self.out)
        self.assertNotEqual(res.returncode, 0)
        self.assertFalse(os.path.exists(self.out))

    def test_unreadable_audio_fails(self) -> None:
        junk = os.path.join(self.tmp.name, "junk.mp3")
        with open(junk, "wb") as f:
            f.write(b"not audio at all")
        for audio in (os.path.join(self.tmp.name, "nope.mp3"), junk):
            res = self.run_script(
                "--audio", audio, "--episode", "7", "--out", self.out, "--model", "tiny",
            )
            self.assertNotEqual(res.returncode, 0, audio)
            self.assertFalse(os.path.exists(self.out), audio)
            self.assertFalse(os.path.exists(self.out + ".tmp"), audio)


class ShiftSegmentsTest(unittest.TestCase):
    def test_offsets_rounds_and_strips(self) -> None:
        segs = [
            SimpleNamespace(start=0.0, end=2.004, text="  привет "),
            SimpleNamespace(start=10.1234, end=12.5, text="мир"),
        ]
        got = transcribe.shift_segments(segs, 60.0, 120.0)
        self.assertEqual(
            got,
            [
                {"start": 60.0, "end": 62.0, "text": "привет"},
                {"start": 70.12, "end": 72.5, "text": "мир"},
            ],
        )

    def test_no_start(self) -> None:
        segs = [SimpleNamespace(start=1.5, end=3.25, text="a")]
        got = transcribe.shift_segments(segs, 0.0, 10.0)
        self.assertEqual(got, [{"start": 1.5, "end": 3.25, "text": "a"}])

    def test_empty(self) -> None:
        self.assertEqual(transcribe.shift_segments([], 5.0, 10.0), [])


if __name__ == "__main__":
    unittest.main()
