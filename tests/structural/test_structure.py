"""Tests for interlab plugin structure."""

import json
import os
import shutil
import stat
import subprocess
import sys
from pathlib import Path

# Add interverse/ to path so _shared package is importable
_interverse = Path(__file__).resolve().parents[3]
if str(_interverse) not in sys.path:
    sys.path.insert(0, str(_interverse))

import pytest

from _shared.tests.structural.test_base import StructuralTests

_HAS_GO = shutil.which("go") is not None


class TestStructure(StructuralTests):
    """Structural tests -- inherits shared base, adds plugin-specific checks."""

    def test_plugin_name(self, plugin_json):
        assert plugin_json["name"] == "interlab"

    def test_mcp_server_config(self, plugin_json):
        """plugin.json has interlab MCP server with stdio type."""
        assert "mcpServers" in plugin_json
        assert "interlab" in plugin_json["mcpServers"]
        server = plugin_json["mcpServers"]["interlab"]
        assert server["type"] == "stdio"
        assert "launch-mcp.sh" in server["command"]

    def test_launch_script_executable(self, project_root):
        """launch-mcp.sh must exist and be executable."""
        path = project_root / "bin" / "launch-mcp.sh"
        assert path.exists(), "launch-mcp.sh not found"
        assert os.access(path, os.X_OK), "launch-mcp.sh is not executable"

    def test_skill_exists(self, project_root):
        """autoresearch skill must have SKILL.md."""
        path = project_root / "skills" / "autoresearch" / "SKILL.md"
        assert path.exists(), "SKILL.md not found"
        content = path.read_text()
        assert len(content) > 100, "SKILL.md seems too short"
        assert "init_experiment" in content
        assert "run_experiment" in content
        assert "log_experiment" in content

    def test_hooks_json_valid(self, project_root):
        """hooks.json must have correct structure."""
        path = project_root / "hooks" / "hooks.json"
        assert path.exists(), "hooks.json not found"
        with open(path) as f:
            data = json.load(f)
        assert "hooks" in data
        assert "SessionStart" in data["hooks"]
        hooks_list = data["hooks"]["SessionStart"]
        assert isinstance(hooks_list, list)
        assert len(hooks_list) > 0

    def test_detect_campaign_executable(self, project_root):
        """detect-campaign.sh must be executable."""
        path = project_root / "hooks" / "detect-campaign.sh"
        assert path.exists()
        assert os.access(path, os.X_OK)

    def test_go_mod_exists(self, project_root):
        """go.mod must exist with correct module name."""
        path = project_root / "go.mod"
        assert path.exists()
        content = path.read_text()
        assert "github.com/mistakeknot/interlab" in content

    @pytest.mark.skipif(not _HAS_GO, reason="Go toolchain not available")
    def test_go_build(self, project_root):
        """Go binary must build successfully."""
        result = subprocess.run(
            ["go", "build", "-o", "/dev/null", "./cmd/interlab-mcp/"],
            cwd=str(project_root),
            capture_output=True,
            text=True,
        )
        assert result.returncode == 0, f"Build failed: {result.stderr}"

    @pytest.mark.skipif(not _HAS_GO, reason="Go toolchain not available")
    def test_go_tests_pass(self, project_root):
        """All Go tests must pass."""
        result = subprocess.run(
            ["go", "test", "./...", "-count=1"],
            cwd=str(project_root),
            capture_output=True,
            text=True,
        )
        assert result.returncode == 0, f"Tests failed: {result.stdout}\n{result.stderr}"

    def test_orchestration_package_exists(self, project_root):
        """Orchestration package must exist with all tool files."""
        orch_dir = project_root / "internal" / "orchestration"
        assert orch_dir.is_dir(), "internal/orchestration/ not found"
        required = ["register.go", "beads.go", "plan.go", "dispatch.go", "status.go", "synthesize.go"]
        for name in required:
            assert (orch_dir / name).exists(), f"Missing orchestration file: {name}"

    def test_orchestration_register_exports(self, project_root):
        """register.go must contain RegisterAll function."""
        path = project_root / "internal" / "orchestration" / "register.go"
        content = path.read_text()
        assert "func RegisterAll" in content
        assert "PlanCampaignsTool" in content
        assert "DispatchCampaignsTool" in content
        assert "StatusCampaignsTool" in content
        assert "SynthesizeCampaignsTool" in content

    def test_autoresearch_multi_skill(self, project_root):
        """/autoresearch-multi skill must exist with SKILL.md."""
        path = project_root / "skills" / "autoresearch-multi" / "SKILL.md"
        assert path.exists(), "autoresearch-multi SKILL.md not found"
        content = path.read_text()
        assert "plan_campaigns" in content
        assert "dispatch_campaigns" in content
        assert "synthesize_campaigns" in content

    def test_plugin_json_skills(self, plugin_json):
        """plugin.json must reference both skills."""
        skills = plugin_json.get("skills", [])
        assert "./skills/autoresearch" in skills
        assert "./skills/autoresearch-multi" in skills
