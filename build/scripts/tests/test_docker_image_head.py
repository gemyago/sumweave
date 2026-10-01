import os
from pathlib import Path
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / "build/scripts/advance-docker-image-head.sh"


class DockerImageHeadTests(unittest.TestCase):
    def setUp(self):
        temp_root = ROOT / "tmp"
        temp_root.mkdir(exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(prefix="docker-head-test-", dir=temp_root)
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)
        self.remote = self.base / "remote.git"
        self.checkout = self.base / "checkout"
        self.checkout.mkdir()
        self.env = os.environ.copy()
        for key in ("SOURCE_SHA", "TAG_NAME", "GIT_REMOTE"):
            self.env.pop(key, None)
        self.env.update({
            "GIT_CONFIG_NOSYSTEM": "1",
            "GIT_CONFIG_GLOBAL": os.devnull,
            "GIT_AUTHOR_NAME": "Local test",
            "GIT_AUTHOR_EMAIL": "test@example.invalid",
            "GIT_COMMITTER_NAME": "Local test",
            "GIT_COMMITTER_EMAIL": "test@example.invalid",
        })
        self.git("init", "--bare", str(self.remote))
        self.git("init", "-b", "main")
        self.git("remote", "add", "origin", str(self.remote))
        self.commits = []
        for name in ("first", "second", "third"):
            self.git("commit", "--allow-empty", "-m", name)
            self.commits.append(self.git("rev-parse", "HEAD"))
        self.git("push", "origin", "main")

    def git(self, *args):
        return subprocess.run(
            ["git", *args], cwd=self.checkout, env=self.env,
            text=True, capture_output=True, check=True,
        ).stdout.strip()

    def run_script(self, source=None, **inputs):
        env = self.env.copy()
        if source is not None:
            env["SOURCE_SHA"] = source
        env.update(inputs)
        return subprocess.run(
            ["bash", str(SCRIPT)], cwd=self.checkout, env=env,
            text=True, capture_output=True,
        )

    def assert_success(self, result):
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def tag(self, commit, name="docker-image-head"):
        self.git("--git-dir", str(self.remote), "update-ref", f"refs/tags/{name}", commit)

    def remote_tag(self, name="docker-image-head"):
        return self.git("--git-dir", str(self.remote), "rev-parse", f"refs/tags/{name}")

    def test_create_default_and_advance(self):
        self.assert_success(self.run_script(self.commits[0]))
        self.assertEqual(self.remote_tag(), self.commits[0])
        self.assert_success(self.run_script(self.commits[1]))
        self.assertEqual(self.remote_tag(), self.commits[1])
        # The updater need not create or overwrite local tags.
        self.assertEqual(self.git("tag", "--list"), "")

    def test_custom_tag_and_remote(self):
        self.git("remote", "rename", "origin", "publication")
        self.assert_success(self.run_script(
            self.commits[1], TAG_NAME="test/image-head", GIT_REMOTE="publication",
        ))
        self.assertEqual(self.remote_tag("test/image-head"), self.commits[1])

    def test_empty_optional_inputs_use_defaults(self):
        self.assert_success(self.run_script(self.commits[0], TAG_NAME="", GIT_REMOTE=""))
        self.assertEqual(self.remote_tag(), self.commits[0])

    def test_equal_and_older_sources_do_not_move_backwards(self):
        self.tag(self.commits[2])
        for source in (self.commits[2], self.commits[0]):
            self.assert_success(self.run_script(source))
            self.assertEqual(self.remote_tag(), self.commits[2])

    def test_divergent_source_does_not_move_tag(self):
        self.tag(self.commits[1])
        self.git("checkout", "-b", "divergent", self.commits[0])
        self.git("commit", "--allow-empty", "-m", "divergent")
        self.assert_success(self.run_script(self.git("rev-parse", "HEAD")))
        self.assertEqual(self.remote_tag(), self.commits[1])

    def test_annotated_tag_uses_commit_ancestry_and_raw_object_lease(self):
        self.git("tag", "-a", "docker-image-head", self.commits[0], "-m", "old head")
        self.git("push", "origin", "refs/tags/docker-image-head")
        self.assert_success(self.run_script(self.commits[1]))
        self.assertEqual(self.remote_tag(), self.commits[1])

    def test_invalid_inputs_fail_without_changing_remote(self):
        self.tag(self.commits[0])
        for source, inputs in (
            (None, {}), ("not-a-commit", {}),
            (self.commits[1], {"TAG_NAME": "bad..tag"}),
            (self.commits[1], {"GIT_REMOTE": "missing"}),
        ):
            with self.subTest(source=source, inputs=inputs):
                self.assertNotEqual(self.run_script(source, **inputs).returncode, 0)
                self.assertEqual(self.remote_tag(), self.commits[0])

    def install_racing_publisher(self):
        # Git's real pre-push hook runs after our observation, before sending the
        # update. Mutate the bare remote there: deterministic race, no sleeps or
        # mocked Git, and no production test hooks.
        hook = self.checkout / ".git/hooks/pre-push"
        hook.write_text(
            '#!/usr/bin/env bash\nset -eu\n'
            f'git --git-dir="{self.remote}" update-ref refs/tags/docker-image-head {self.commits[2]}\n'
        )
        hook.chmod(0o755)

    def test_remote_option_cannot_execute_upload_pack(self):
        self.tag(self.commits[0])
        marker = self.base / "remote-option-executed"
        remote = f"--upload-pack=touch '{marker}'; git-upload-pack"
        result = self.run_script(self.commits[1], GIT_REMOTE=remote)
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(marker.exists(), "remote input executed an upload-pack command")
        self.assertEqual(self.remote_tag(), self.commits[0])

    def test_observation_before_advertisement_race_requires_explicit_lease(self):
        self.tag(self.commits[0])
        # The real upload-pack returns the observed old tag, then advances it
        # before push advertisement. Plain --force would overwrite this winner;
        # a post-advertisement pre-push race alone would not detect that bug.
        wrapper = self.base / "upload-pack-wrapper"
        wrapper.write_text(
            '#!/usr/bin/env bash\nset -eu\n'
            'git-upload-pack "$@"\n'
            f'git --git-dir="{self.remote}" update-ref refs/tags/docker-image-head {self.commits[2]}\n'
        )
        wrapper.chmod(0o755)
        self.git("config", "remote.origin.uploadpack", str(wrapper))
        result = self.run_script(self.commits[1])
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.remote_tag(), self.commits[2])
        self.assertIn("stale info", result.stderr)

    def test_initial_creation_lease_rejects_racing_publisher(self):
        self.install_racing_publisher()
        result = self.run_script(self.commits[1])
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.remote_tag(), self.commits[2])

    def test_existing_tag_lease_rejects_racing_publisher(self):
        self.tag(self.commits[0])
        self.install_racing_publisher()
        result = self.run_script(self.commits[1])
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.remote_tag(), self.commits[2])


if __name__ == "__main__":
    unittest.main()
