# Copied from gemyago/golang-backend-boilerplate@798f0dc9fd753481d0d698d8232ea08df44185b6 and focused on Sumweave retention policy.
import datetime
import pathlib
import sys
import unittest

sys.path.insert(0, str(pathlib.Path(__file__).parents[1]))
from ghcr import cleanup_actions


class CleanupActionsTest(unittest.TestCase):
    def test_retains_release_tags_and_manifest_children(self):
        now = datetime.datetime(2026, 7, 16, tzinfo=datetime.UTC)
        def version(identifier, created, tags):
            return {"id": identifier, "created_at": created.isoformat(), "metadata": {"container": {"tags": tags}}}
        actions = cleanup_actions([
            version(1, now - datetime.timedelta(days=90), ["latest"]),
            version(2, now - datetime.timedelta(days=90, seconds=-3), []),
            version(3, now - datetime.timedelta(days=8), ["feature-a"]),
            version(4, now - datetime.timedelta(days=8), ["git-commit-abcdef0"]),
        ], 7 * 24 * 60 * 60, r"^(latest|latest-|git-tag-|v[0-9])", now)
        self.assertEqual([(item[0]["id"], item[1]) for item in actions], [(1, True), (2, True), (3, False), (4, False)])
