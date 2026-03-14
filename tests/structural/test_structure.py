"""Structural tests for interlab plugin."""
import json
import os
import stat
import subprocess

PLUGIN_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))


def test_plugin_json_valid():
    """plugin.json must be valid JSON with required fields."""
    path = os.path.join(PLUGIN_ROOT, ".claude-plugin", "plugin.json")
    assert os.path.exists(path), f"plugin.json not found at {path}"
    with open(path) as f:
        data = json.load(f)
    assert "name" in data
    assert data["name"] == "interlab"
    assert "mcpServers" in data
    assert "interlab" in data["mcpServers"]
    server = data["mcpServers"]["interlab"]
    assert server["type"] == "stdio"
    assert "launch-mcp.sh" in server["command"]


def test_launch_script_executable():
    """launch-mcp.sh must exist and be executable."""
    path = os.path.join(PLUGIN_ROOT, "bin", "launch-mcp.sh")
    assert os.path.exists(path), f"launch-mcp.sh not found"
    mode = os.stat(path).st_mode
    assert mode & stat.S_IXUSR, "launch-mcp.sh is not executable"


def test_skill_exists():
    """autoresearch skill must have SKILL.md."""
    path = os.path.join(PLUGIN_ROOT, "skills", "autoresearch", "SKILL.md")
    assert os.path.exists(path), f"SKILL.md not found"
    with open(path) as f:
        content = f.read()
    assert len(content) > 100, "SKILL.md seems too short"
    assert "init_experiment" in content
    assert "run_experiment" in content
    assert "log_experiment" in content


def test_hooks_json_valid():
    """hooks.json must have correct structure."""
    path = os.path.join(PLUGIN_ROOT, "hooks", "hooks.json")
    assert os.path.exists(path), f"hooks.json not found"
    with open(path) as f:
        data = json.load(f)
    assert "hooks" in data
    assert "SessionStart" in data["hooks"]
    hooks_list = data["hooks"]["SessionStart"]
    assert isinstance(hooks_list, list)
    assert len(hooks_list) > 0


def test_detect_campaign_executable():
    """detect-campaign.sh must be executable."""
    path = os.path.join(PLUGIN_ROOT, "hooks", "detect-campaign.sh")
    assert os.path.exists(path)
    mode = os.stat(path).st_mode
    assert mode & stat.S_IXUSR


def test_go_mod_exists():
    """go.mod must exist with correct module name."""
    path = os.path.join(PLUGIN_ROOT, "go.mod")
    assert os.path.exists(path)
    with open(path) as f:
        content = f.read()
    assert "github.com/mistakeknot/interlab" in content


def test_required_files_exist():
    """Standard plugin files must exist."""
    required = ["CLAUDE.md", "README.md", "LICENSE", ".gitignore"]
    for name in required:
        path = os.path.join(PLUGIN_ROOT, name)
        assert os.path.exists(path), f"Required file missing: {name}"


def test_go_build():
    """Go binary must build successfully."""
    result = subprocess.run(
        ["go", "build", "-o", "/dev/null", "./cmd/interlab-mcp/"],
        cwd=PLUGIN_ROOT,
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, f"Build failed: {result.stderr}"


def test_go_tests_pass():
    """All Go tests must pass."""
    result = subprocess.run(
        ["go", "test", "./...", "-count=1"],
        cwd=PLUGIN_ROOT,
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, f"Tests failed: {result.stdout}\n{result.stderr}"


def test_orchestration_package_exists():
    """Orchestration package must exist with all tool files."""
    orch_dir = os.path.join(PLUGIN_ROOT, "internal", "orchestration")
    assert os.path.isdir(orch_dir), "internal/orchestration/ not found"
    required = ["register.go", "beads.go", "plan.go", "dispatch.go", "status.go", "synthesize.go"]
    for name in required:
        path = os.path.join(orch_dir, name)
        assert os.path.exists(path), f"Missing orchestration file: {name}"


def test_orchestration_register_exports():
    """register.go must contain RegisterAll function."""
    path = os.path.join(PLUGIN_ROOT, "internal", "orchestration", "register.go")
    with open(path) as f:
        content = f.read()
    assert "func RegisterAll" in content
    assert "PlanCampaignsTool" in content
    assert "DispatchCampaignsTool" in content
    assert "StatusCampaignsTool" in content
    assert "SynthesizeCampaignsTool" in content


def test_autoresearch_multi_skill():
    """/autoresearch-multi skill must exist with SKILL.md."""
    path = os.path.join(PLUGIN_ROOT, "skills", "autoresearch-multi", "SKILL.md")
    assert os.path.exists(path), "autoresearch-multi SKILL.md not found"
    with open(path) as f:
        content = f.read()
    assert "plan_campaigns" in content
    assert "dispatch_campaigns" in content
    assert "synthesize_campaigns" in content


def test_plugin_json_skills():
    """plugin.json must reference both skills."""
    path = os.path.join(PLUGIN_ROOT, ".claude-plugin", "plugin.json")
    with open(path) as f:
        data = json.load(f)
    skills = data.get("skills", [])
    assert "./skills/autoresearch" in skills
    assert "./skills/autoresearch-multi" in skills
