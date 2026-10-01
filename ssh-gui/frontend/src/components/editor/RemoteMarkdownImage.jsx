import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ReadRemoteFile } from "../../../bindings/ssh-gui/app";

const imageTypes = {
  png: "image/png", jpg: "image/jpeg", jpeg: "image/jpeg",
  gif: "image/gif", webp: "image/webp", bmp: "image/bmp", svg: "image/svg+xml",
};

export default function RemoteMarkdownImage({ src, alt, title, width, height, className, style, align, sessionId, path }) {
  const { t } = useTranslation();
  const [image, setImage] = useState(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let active = true;
    let objectUrl;
    setImage(null);
    setFailed(false);

    const load = async () => {
      try {
        if (!src) throw new Error("Missing image source");
        if (/^(https?:)?\/\//i.test(src)) {
          if (active) setImage(src.startsWith("//") ? `https:${src}` : src);
          return;
        }
        if (/^[a-z][a-z\d+.-]*:/i.test(src)) throw new Error("Unsupported image source");
        const directory = path.slice(0, path.lastIndexOf("/") + 1);
        const remotePath = decodeURIComponent(new URL(src, `https://remote.invalid${directory}`).pathname);
        const extension = remotePath.split(".").pop().toLowerCase();
        const type = imageTypes[extension];
        if (!type || !sessionId) throw new Error("Unsupported remote image");
        const content = await ReadRemoteFile(sessionId, remotePath);
        if (!active) return;
        const bytes = extension === "svg"
          ? content
          : Uint8Array.from(atob(content), (character) => character.charCodeAt(0));
        objectUrl = URL.createObjectURL(new Blob([bytes], { type }));
        setImage(objectUrl);
      } catch {
        if (active) setFailed(true);
      }
    };

    load();
    return () => {
      active = false;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [src, sessionId, path]);

  if (failed) return <span className="markdown-image-status">{t("files.imageUnavailable")}{alt ? `: ${alt}` : ""}</span>;
  if (!image) return <span className="markdown-image-status">{t("common.loading")}</span>;
  return <img src={image} alt={alt || ""} title={title} width={width} height={height} className={className} style={style} align={align} onError={() => setFailed(true)} />;
}
