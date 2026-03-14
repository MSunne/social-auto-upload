from datetime import datetime
import mimetypes
from pathlib import Path


TEXT_EXTENSIONS = {
    ".txt",
    ".md",
    ".markdown",
    ".json",
    ".csv",
    ".tsv",
    ".yaml",
    ".yml",
    ".toml",
    ".py",
    ".js",
    ".ts",
    ".jsx",
    ".tsx",
    ".html",
    ".css",
    ".scss",
    ".less",
    ".xml",
    ".sql",
    ".log",
}


def build_material_roots(base_dir, configured_roots=None):
    base_dir = Path(base_dir).resolve()
    defaults = {
        "videoFile": base_dir / "videoFile",
        "taskArtifacts": base_dir / "taskArtifacts",
    }

    openclaw_workspace = Path.home() / ".openclaw" / "workspace"
    if openclaw_workspace.exists():
        defaults["openclawWorkspace"] = openclaw_workspace

    materials_dir = base_dir / "materials"
    if materials_dir.exists():
        defaults["materials"] = materials_dir

    for name, path in _coerce_root_items(base_dir, configured_roots):
        defaults[str(name).strip()] = path

    roots = {}
    for name, path in defaults.items():
        root_path = Path(path).expanduser()
        if not root_path.is_absolute():
            root_path = (base_dir / root_path).resolve()
        else:
            root_path = root_path.resolve()
        roots[name] = root_path
    return roots


def list_material_roots(root_map):
    roots = []
    for name, root_path in root_map.items():
        roots.append(
            {
                "name": name,
                "path": str(root_path),
                "exists": root_path.exists(),
                "isDirectory": root_path.is_dir(),
            }
        )
    return roots


def resolve_material_reference(root_map, root_name=None, relative_path=None, absolute_path=None):
    if absolute_path:
        absolute_candidate = Path(absolute_path).expanduser().resolve()
        for name, root_path in root_map.items():
            if _is_relative_to(absolute_candidate, root_path):
                return {
                    "rootName": name,
                    "rootPath": root_path,
                    "relativePath": absolute_candidate.relative_to(root_path).as_posix(),
                    "absolutePath": str(absolute_candidate),
                }
        raise ValueError("文件路径不在允许的素材目录内")

    if not root_name:
        raise ValueError("缺少素材根目录名称")

    root_path = root_map.get(root_name)
    if root_path is None:
        raise ValueError(f"未知素材根目录: {root_name}")

    relative_value = str(relative_path or "").strip().replace("\\", "/").lstrip("/")
    candidate = (root_path / relative_value).resolve()
    if not _is_relative_to(candidate, root_path):
        raise ValueError("非法素材路径")

    return {
        "rootName": root_name,
        "rootPath": root_path,
        "relativePath": relative_value,
        "absolutePath": str(candidate),
    }


def list_material_directory(root_map, root_name, relative_path="", limit=200):
    limit = max(1, min(int(limit), 1000))
    resolved = resolve_material_reference(root_map, root_name=root_name, relative_path=relative_path)
    target = Path(resolved["absolutePath"])

    if not target.exists():
        raise FileNotFoundError("素材目录不存在")
    if not target.is_dir():
        raise NotADirectoryError("目标路径不是目录")

    entries = []
    for entry in sorted(target.iterdir(), key=lambda item: (not item.is_dir(), item.name.lower()))[:limit]:
        stat = entry.stat()
        mime_type, _ = mimetypes.guess_type(entry.name)
        entries.append(
            {
                "name": entry.name,
                "kind": "directory" if entry.is_dir() else "file",
                "relativePath": entry.relative_to(resolved["rootPath"]).as_posix(),
                "absolutePath": str(entry),
                "size": stat.st_size,
                "modifiedAt": _format_mtime(stat.st_mtime),
                "extension": entry.suffix.lower(),
                "mimeType": mime_type,
            }
        )

    return {
        "root": resolved["rootName"],
        "rootPath": str(resolved["rootPath"]),
        "path": resolved["relativePath"],
        "absolutePath": str(target),
        "entries": entries,
    }


def read_material_file(root_map, root_name, relative_path, max_bytes=65536):
    max_bytes = max(1024, min(int(max_bytes), 1024 * 1024))
    resolved = resolve_material_reference(root_map, root_name=root_name, relative_path=relative_path)
    target = Path(resolved["absolutePath"])

    if not target.exists():
        raise FileNotFoundError("素材文件不存在")
    if not target.is_file():
        raise IsADirectoryError("目标路径不是文件")

    stat = target.stat()
    mime_type, _ = mimetypes.guess_type(target.name)
    with open(target, "rb") as file_obj:
        raw = file_obj.read(max_bytes + 1)

    truncated = len(raw) > max_bytes
    if truncated:
        raw = raw[:max_bytes]

    is_text = _looks_like_text(target, mime_type, raw)
    preview_text = None
    if is_text:
        preview_text = raw.decode("utf-8", errors="replace")

    return {
        "root": resolved["rootName"],
        "rootPath": str(resolved["rootPath"]),
        "path": resolved["relativePath"],
        "absolutePath": str(target),
        "name": target.name,
        "size": stat.st_size,
        "modifiedAt": _format_mtime(stat.st_mtime),
        "mimeType": mime_type,
        "isText": is_text,
        "truncated": truncated,
        "previewText": preview_text,
        "extension": target.suffix.lower(),
    }


def _coerce_root_items(base_dir, configured_roots):
    if not configured_roots:
        return []

    items = []
    if isinstance(configured_roots, dict):
        for name, value in configured_roots.items():
            items.append((name, _resolve_root_path(base_dir, value)))
        return items

    if not isinstance(configured_roots, (list, tuple, set)):
        configured_roots = [configured_roots]

    for index, item in enumerate(configured_roots):
        if isinstance(item, (list, tuple)) and len(item) == 2:
            name, value = item
        else:
            value = item
            name = Path(str(item)).expanduser().name or f"root{index + 1}"
        items.append((str(name).strip(), _resolve_root_path(base_dir, value)))
    return items


def _resolve_root_path(base_dir, value):
    path = Path(value).expanduser()
    if not path.is_absolute():
        path = (Path(base_dir) / path).resolve()
    else:
        path = path.resolve()
    return path


def _is_relative_to(path, root_path):
    try:
        path.relative_to(root_path)
        return True
    except ValueError:
        return False


def _looks_like_text(path, mime_type, raw):
    if path.suffix.lower() in TEXT_EXTENSIONS:
        return True
    if mime_type and mime_type.startswith("text/"):
        return True
    if b"\x00" in raw:
        return False
    try:
        raw.decode("utf-8")
        return True
    except UnicodeDecodeError:
        return False


def _format_mtime(timestamp):
    return datetime.fromtimestamp(timestamp).strftime("%Y-%m-%d %H:%M:%S")
