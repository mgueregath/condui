export function registerEnvLanguage(monaco) {
  if (monaco.languages.getLanguages().some(({ id }) => id === "dotenv")) return;

  monaco.languages.register({ id: "dotenv" });
  monaco.languages.setLanguageConfiguration("dotenv", {
    comments: { lineComment: "#" },
  });
  monaco.languages.setMonarchTokensProvider("dotenv", {
    tokenizer: {
      root: [
        [/^\s*#.*$/, "comment"],
        [/^(\s*)(export)(\s+)/, ["", "keyword", ""]],
        [/([A-Za-z_][\w.-]*)(\s*)(=)/, ["variable", "", { token: "delimiter", next: "@value" }]],
      ],
      value: [
        [/^/, { token: "@rematch", next: "@pop" }],
        [/\s+#.*$/, "comment", "@pop"],
        [/"/, "string", "@doubleQuoted"],
        [/'/, "string", "@singleQuoted"],
        [/\$\{[^}]*\}|\$[A-Za-z_]\w*/, "variable"],
        [/[^\s"'$]+/, "string"],
        [/\s+/, ""],
        [/./, "string"],
      ],
      doubleQuoted: [
        [/\\./, "string.escape"],
        [/\$\{[^}]*\}|\$[A-Za-z_]\w*/, "variable"],
        [/"/, "string", "@pop"],
        [/[^"\\$]+/, "string"],
        [/./, "string"],
      ],
      singleQuoted: [
        [/'/, "string", "@pop"],
        [/[^']+/, "string"],
      ],
    },
  });
}
