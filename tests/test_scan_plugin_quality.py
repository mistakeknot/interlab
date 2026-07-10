"""Regression tests for the plugin quality scanner."""

import json
import os
import subprocess
from pathlib import Path

import pytest


SCANNER = Path(__file__).resolve().parents[1] / "scripts" / "scan-plugin-quality.sh"


def _run_scanner(
    tmp_path: Path, metrics: str, *, benchmark_exit: int = 0
) -> subprocess.CompletedProcess[str]:
    root = tmp_path / "demarch"
    scripts_dir = root / "interverse" / "interlab" / "scripts"
    plugin_dir = root / "interverse" / "example" / ".claude-plugin"
    fake_bin = tmp_path / "bin"

    scripts_dir.mkdir(parents=True)
    plugin_dir.mkdir(parents=True)
    fake_bin.mkdir()

    (plugin_dir / "plugin.json").write_text('{"name": "example"}\n')
    benchmark = scripts_dir / "plugin-benchmark.sh"
    benchmark.write_text(
        "#!/usr/bin/env bash\n"
        "cat <<'METRICS'\n"
        f"{metrics.rstrip()}\n"
        "METRICS\n"
        f"exit {benchmark_exit}\n"
    )

    # The scanner must not depend on GNU grep extensions. A fake grep makes the
    # regression deterministic on both GNU/Linux and macOS runners.
    fake_grep = fake_bin / "grep"
    fake_grep.write_text("#!/usr/bin/env bash\nexit 97\n")
    fake_grep.chmod(0o755)

    env = os.environ.copy()
    env["DEMARCH_ROOT"] = str(root)
    env["PATH"] = f"{fake_bin}{os.pathsep}{env['PATH']}"
    return subprocess.run(
        ["bash", str(SCANNER), "--json"],
        capture_output=True,
        text=True,
        env=env,
        cwd=root,
    )


def test_scanner_parses_metrics_without_gnu_grep(tmp_path):
    result = _run_scanner(
        tmp_path,
        """\
METRIC structural_tests_pass=8
METRIC structural_tests_total=10
METRIC build_passes=1
METRIC audit_score=15
METRIC audit_max=20
METRIC plugin_quality_score=0.625
""",
    )

    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["total_plugins"] == 1
    assert payload["avg_pqs"] == pytest.approx(0.625)
    assert payload["all"][0] == {
        "name": "example",
        "path": f"{tmp_path}/demarch/interverse/example/",
        "pqs": pytest.approx(0.625),
        "audit_score": 15,
        "audit_max": 20,
        "structural_pass": 8,
        "structural_total": 10,
        "build_passes": 1,
        "has_interlab": 0,
    }


def test_scanner_rejects_missing_metric(tmp_path):
    result = _run_scanner(
        tmp_path,
        """\
METRIC structural_tests_pass=8
METRIC structural_tests_total=10
METRIC build_passes=1
METRIC audit_score=15
METRIC audit_max=20
""",
    )

    assert result.returncode != 0
    assert "example" in result.stderr
    assert "plugin_quality_score" in result.stderr


def test_scanner_rejects_nonnumeric_metric(tmp_path):
    result = _run_scanner(
        tmp_path,
        """\
METRIC structural_tests_pass=8
METRIC structural_tests_total=10
METRIC build_passes=unknown
METRIC audit_score=15
METRIC audit_max=20
METRIC plugin_quality_score=0.625
""",
    )

    assert result.returncode != 0
    assert "example" in result.stderr
    assert "build_passes" in result.stderr


def test_scanner_rejects_duplicate_metric(tmp_path):
    result = _run_scanner(
        tmp_path,
        """\
METRIC structural_tests_pass=8
METRIC structural_tests_total=10
METRIC build_passes=1
METRIC audit_score=15
METRIC audit_max=20
METRIC plugin_quality_score=0.625
METRIC plugin_quality_score=0.750
""",
    )

    assert result.returncode != 0
    assert "example" in result.stderr
    assert "plugin_quality_score" in result.stderr


def test_scanner_rejects_benchmark_failure(tmp_path):
    result = _run_scanner(tmp_path, "benchmark failed", benchmark_exit=42)

    assert result.returncode != 0
    assert "benchmark failed for plugin example" in result.stderr
