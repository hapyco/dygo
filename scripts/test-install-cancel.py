#!/usr/bin/env python3
"""Exercise parent-only installer cancellation with real stalled curl requests."""

import errno
import http.server
import os
from pathlib import Path
import pty
import select
import shutil
import signal
import subprocess
import sys
import tempfile
import threading
import time
import unittest


INSTALLER = Path(sys.argv.pop(1)).resolve() if len(sys.argv) > 1 else Path(__file__).with_name("install.sh").resolve()


class InstallerCancellationTest(unittest.TestCase):
    def test_stalled_downloads(self):
        for stage in ("latest", "archive", "checksums"):
            for sig in (signal.SIGINT, signal.SIGTERM):
                with self.subTest(stage=stage, signal=sig.name):
                    self.check_cancellation(stage, sig)

    def check_cancellation(self, stage, sig):
        arrived = threading.Event()
        unrelated_arrived = threading.Event()
        disconnected = threading.Event()
        release = threading.Event()
        requests = []

        class Handler(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                if self.path != "/unrelated":
                    requests.append(self.path)
                if stage == "checksums" and self.path.endswith(".tar.gz"):
                    # Verification happens after both downloads, so this need not be a valid archive.
                    self.send_response(200)
                    self.send_header("Content-Length", "7")
                    self.end_headers()
                    self.wfile.write(b"fixture")
                    return
                (unrelated_arrived if self.path == "/unrelated" else arrived).set()
                deadline = time.monotonic() + 10
                while not release.wait(0.02) and time.monotonic() < deadline:
                    if select.select([self.connection], [], [], 0)[0]:
                        if not self.connection.recv(1):
                            if self.path != "/unrelated":
                                disconnected.set()
                            return

            def log_message(self, *_):
                pass

        server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        server_thread = threading.Thread(target=server.serve_forever, kwargs={"poll_interval": 0.02})
        server_thread.start()
        proc = unrelated = None
        terminal = None
        with tempfile.TemporaryDirectory(prefix="dygo-cancel-test-") as directory:
            root = Path(directory)
            install_dir = root / "install"
            install_dir.mkdir()
            previous = install_dir / "dygo"
            previous.write_text('#!/bin/sh\nprintf "dygo v0.0.6\\n"\n')
            previous.chmod(0o755)
            old_binary = previous.read_bytes()
            temp_dir = root / "downloads"
            temp_dir.mkdir()
            helpers = root / "helpers"
            helpers.mkdir()
            curl_pids = root / "curl-pids"
            # Exec real curl, retaining its PID. Redirect only the hard-coded latest metadata URL.
            (helpers / "curl").write_text(
                "#!/usr/bin/env python3\n"
                "import os, sys\n"
                "with open(os.environ['DYGO_TEST_CURL_PIDS'], 'a') as output:\n"
                "    output.write(str(os.getpid()) + '\\n')\n"
                "args = [os.environ['DYGO_TEST_LATEST_URL'] if arg == "
                "'https://api.github.com/repos/hapyco/dygo/releases/latest' else arg for arg in sys.argv[1:]]\n"
                "os.execv(os.environ['DYGO_TEST_REAL_CURL'], ['curl'] + args)\n"
            )
            (helpers / "curl").chmod(0o755)
            base = f"http://127.0.0.1:{server.server_port}"
            real_curl = shutil.which("curl")
            self.assertIsNotNone(real_curl)
            env = dict(os.environ, DYGO_VERSION="latest" if stage == "latest" else "v0.0.8",
                       DYGO_INSTALL_DIR=str(install_dir), DYGO_DOWNLOAD_BASE_URL=base,
                       DYGO_TEST_CURL_PIDS=str(curl_pids), DYGO_TEST_LATEST_URL=base + "/latest",
                       DYGO_TEST_REAL_CURL=real_curl, TMPDIR=str(temp_dir),
                       PATH=str(helpers) + os.pathsep + os.environ["PATH"])
            interactive = stage == "archive" and sig == signal.SIGTERM
            stderr = subprocess.PIPE
            captured = b""
            if interactive:
                env.pop("CI", None)
                env["TERM"] = "xterm"
                terminal, stderr = pty.openpty()
            else:
                env["CI"] = "true"
            try:
                unrelated = subprocess.Popen([real_curl, "-fsS", base + "/unrelated"],
                                             stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                self.assertTrue(unrelated_arrived.wait(3), "unrelated curl did not reach server")
                proc = subprocess.Popen(["/bin/sh", str(INSTALLER)], env=env,
                                        stdout=subprocess.PIPE, stderr=stderr, start_new_session=True)
                if interactive:
                    os.close(stderr)
                self.assertTrue(arrived.wait(3), "installer did not reach stalled HTTP request")
                if interactive:
                    deadline = time.monotonic() + 3
                    while b"Downloading dygo" not in captured and time.monotonic() < deadline:
                        if select.select([terminal], [], [], 0.1)[0]:
                            captured += os.read(terminal, 4096)
                    self.assertIn(b"Downloading dygo", captured, "spinner did not start")
                proc.send_signal(sig)  # Signal only the installer PID, not its process group.
                stdout, _ = proc.communicate(timeout=2)
                self.assertEqual(proc.returncode, 128 + sig)
                self.assertEqual(stdout, b"")
                self.assertTrue(disconnected.wait(1), "owned HTTP connection stayed open")
                for pid in curl_pids.read_text().splitlines():
                    with self.assertRaises(ProcessLookupError, msg="owned curl was not reaped"):
                        os.kill(int(pid), 0)
                self.assertIsNone(unrelated.poll(), "installer terminated unrelated curl")
                self.assertEqual(previous.read_bytes(), old_binary)
                self.assertEqual(list(install_dir.iterdir()), [previous])
                self.assertEqual(list(temp_dir.iterdir()), [], "download temporary directory remains")
                self.assertEqual(len(requests), 2 if stage == "checksums" else 1,
                                 "installer continued to a subsequent download")
                if interactive:
                    deadline = time.monotonic() + 2
                    while time.monotonic() < deadline:
                        if select.select([terminal], [], [], 0.1)[0]:
                            try:
                                chunk = os.read(terminal, 4096)
                            except OSError as error:
                                if error.errno != errno.EIO:
                                    raise
                                break
                            if not chunk:
                                break
                            captured += chunk
                    else:
                        self.fail("progress process kept stderr open after cancellation")
                    # Native shells may append a termination diagnostic after clearing the line.
                    # Every spinner frame must precede the final clear, regardless of that suffix.
                    message = b"Downloading dygo"
                    self.assertIn(message, captured, f"spinner output was not captured: {captured!r}")
                    final_clear = captured.rfind(b"\r\x1b[2K")
                    last_frame_end = captured.rfind(message) + len(message)
                    self.assertGreaterEqual(final_clear, last_frame_end,
                                            f"spinner line was not cleared after its last frame: {captured!r}")
            finally:
                release.set()
                if proc is not None and proc.poll() is None:
                    # Only the isolated test-owned session is killed after a failed assertion.
                    os.killpg(proc.pid, signal.SIGKILL)
                    proc.communicate(timeout=3)
                if unrelated is not None:
                    unrelated.terminate()
                    unrelated.wait(timeout=3)
                if terminal is not None:
                    os.close(terminal)
                server.shutdown()
                server.server_close()
                server_thread.join(timeout=3)


if __name__ == "__main__":
    unittest.main()
