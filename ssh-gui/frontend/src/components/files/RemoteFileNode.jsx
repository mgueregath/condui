import {
  FaFolder, FaFile, FaFileAudio, FaFileCode, FaFileCsv, FaFileExcel,
  FaFileImage, FaFileLines, FaFilePdf, FaFilePowerpoint, FaFileVideo,
  FaFileWord, FaFileZipper,
} from "react-icons/fa6";
import { useTranslation } from "react-i18next";

const fileIcons = Object.fromEntries([
  [FaFileImage, "jpg jpeg png gif webp svg ico bmp tiff tif avif heic"],
  [FaFileAudio, "mp3 wav ogg flac aac m4a wma opus aiff"],
  [FaFileVideo, "mp4 mkv avi mov webm m4v wmv mpeg mpg"],
  [FaFileZipper, "zip rar 7z tar gz bz2 xz tgz zst"],
  [FaFilePdf, "pdf"],
  [FaFileWord, "doc docx odt rtf"],
  [FaFileExcel, "xls xlsx ods"],
  [FaFilePowerpoint, "ppt pptx odp"],
  [FaFileCsv, "csv tsv"],
  [FaFileLines, "txt md markdown log rst"],
  [FaFileCode, "js jsx ts tsx mjs cjs html htm css scss sass less json jsonc xml yaml yml toml ini conf cfg env py go rs java c h cpp hpp cs php rb sh bash zsh fish ps1 bat cmd sql vue svelte dockerfile"],
].flatMap(([Icon, extensions]) => extensions.split(" ").map((extension) => [extension, Icon])));

function getFileIcon(name) {
  const normalizedName = name.toLowerCase();
  if (["dockerfile", "makefile", "jenkinsfile"].includes(normalizedName)) return FaFileCode;
  return fileIcons[normalizedName.split(".").pop()] || FaFile;
}

function fmtSize(bytes, isDir, folderLabel) {
  if (isDir) return folderLabel;
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log2(Math.max(bytes, 1)) / 10);
  const idx = Math.min(i, units.length - 1);
  const val = bytes / Math.pow(1024, idx);
  return (idx === 0 ? val.toFixed(0) : val.toFixed(1)) + " " + units[idx];
}

function fmtDate(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "";
  return d.toLocaleDateString(undefined, { day: "2-digit", month: "short", year: "numeric" });
}

export default function RemoteFileNode({ item, onOpen, onOpenFile, onContextMenu }) {
  const { t, i18n } = useTranslation();
  const isDir = item.isDirectory;
  const Icon = isDir ? FaFolder : getFileIcon(item.name);

  return (
    <div
      className={"file-node" + (item.name.startsWith(".") ? " is-hidden" : "")}
      onDoubleClick={() => {
        if (isDir) onOpen(item);
        else onOpenFile(item);
      }}
      onContextMenu={(e) => {
        e.preventDefault();
        onContextMenu?.(item, e.clientX, e.clientY);
      }}
    >
      <div className={"file-node-icon" + (isDir ? " dir" : "")}>
        <Icon />
      </div>
      <div className="file-node-info">
        <span className="file-node-name">{item.name}</span>
        <span className="file-node-meta">
          {fmtSize(item.size, isDir, t("files.folder"))}
          {item.modTime && <span className="file-node-meta-sep">·</span>}
          {item.modTime && new Date(item.modTime).toLocaleDateString(i18n.language, { day: "2-digit", month: "short", year: "numeric" })}
        </span>
      </div>
    </div>
  );
}
