import re
from pathlib import Path

from fastapi import APIRouter, HTTPException
from fastapi.responses import FileResponse

from backend.app.common.settings import get_settings

router = APIRouter(tags=["agent distribution"])
installer = Path(__file__).resolve().parents[3] / "agent" / "installer" / "install-agent.sh"


@router.get("/install-agent", include_in_schema=False)
def install_agent() -> FileResponse:
    return FileResponse(installer, media_type="text/x-shellscript", filename="install-agent.sh")


@router.get("/agent/releases/{version}/{filename}", include_in_schema=False)
def agent_release(version: str, filename: str) -> FileResponse:
    # The release directory holds exactly what the image build put there, so any agent's
    # artifacts are served from it. The names are bounded only so that neither segment can
    # walk out of the directory.
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", version):
        raise HTTPException(status_code=404, detail="release not found")
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*", filename) or ".." in filename:
        raise HTTPException(status_code=404, detail="release not found")
    path = Path(get_settings().agent_release_dir) / version / filename
    if not path.is_file():
        raise HTTPException(status_code=404, detail="release not found")
    return FileResponse(path, filename=filename)
