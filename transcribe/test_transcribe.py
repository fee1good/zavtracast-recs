import json
import os
import subprocess
import sys
import tempfile
import unittest
import wave

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

    def test_missing_audio_fails(self) -> None:
        res = self.run_script("--episode", "7", "--out", self.out)
        self.assertNotEqual(res.returncode, 0)
        self.assertFalse(os.path.exists(self.out))


if __name__ == "__main__":
    unittest.main()
