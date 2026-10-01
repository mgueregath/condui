import { defaultSchema } from "rehype-sanitize";

// Allow presentation styles without URLs, positioning or executable CSS values.
const styleProperty = "(?:color|background-color|font-size|font-weight|font-style|font-family|text-align|text-decoration|line-height|letter-spacing|width|height|max-width|max-height|margin(?:-(?:top|right|bottom|left))?|padding(?:-(?:top|right|bottom|left))?|border(?:-(?:color|style|width|radius))?)";
const styleValue = "(?:[-a-z\\d#.% ,]+|(?:rgb|rgba|hsl|hsla)\\([\\d.% ,+-]+\\)|var\\(--editor-[a-z-]+\\))";
const declaration = `(?:${styleProperty}\\s*:\\s*${styleValue}|display\\s*:\\s*(?:inline-block|inline|block))`;
const inlineStyle = new RegExp(`^\\s*(?:${declaration}\\s*;\\s*)*(?:${declaration}\\s*;?\\s*)?$`, "i");

const attributes = Object.fromEntries(
  Object.entries(defaultSchema.attributes).map(([tag, rules]) => [
    tag,
    rules.filter((rule) => (Array.isArray(rule) ? rule[0] : rule) !== "className"),
  ]),
);

export default {
  ...defaultSchema,
  strip: [...defaultSchema.strip, "style", "iframe", "object", "embed"],
  attributes: {
    ...attributes,
    "*": [
      ...attributes["*"],
      ["className", /^[a-z][a-z\d_-]*$/i],
      ["style", inlineStyle],
    ],
  },
};
