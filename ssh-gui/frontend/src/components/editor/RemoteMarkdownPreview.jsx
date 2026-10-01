import { useMemo } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeRaw from "rehype-raw";
import rehypeSanitize from "rehype-sanitize";
import markdownHtmlSchema from "./markdownHtmlSchema";
import RemoteMarkdownImage from "./RemoteMarkdownImage";
import "./RemoteMarkdownPreview.css";

export default function RemoteMarkdownPreview({ content, path, sessionId }) {
  const components = useMemo(() => ({
    img: ({ src, alt, title, width, height, className, style, align }) => (
      <RemoteMarkdownImage src={src} alt={alt} title={title} width={width} height={height} className={className} style={style} align={align} path={path} sessionId={sessionId} />
    ),
    a: ({ href, children, className, style, title }) => {
      if (!/^https?:\/\//i.test(href || "")) return <span className={className} style={style} title={title}>{children}</span>;
      return <a href={href} className={className} style={style} title={title} target="_blank" rel="noopener noreferrer">{children}</a>;
    },
  }), [path, sessionId]);

  return (
    <div className="remote-markdown-preview">
      <Markdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeRaw, [rehypeSanitize, markdownHtmlSchema]]}
        components={components}
      >
        {content || ""}
      </Markdown>
    </div>
  );
}
