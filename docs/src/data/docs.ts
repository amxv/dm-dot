export const siteConfig = {
  name: "dm-dot",
  strapline: "A direct line to your ChatGPT dot",
  description:
    "Send one-way messages to a subscribed ChatGPT dot using MCP Events or the authenticated REST API.",
  repoUrl: "https://github.com/amxv/dm-dot",
  accentColor: "#397850",
  accentColorDark: "#ace58d",
  footerSections: [
    {
      title: "dm-dot",
      text: "One small, event-driven bridge between your agents and a subscribed ChatGPT dot."
    },
    {
      title: "Integrations",
      text: "One MCP tool, one authenticated REST endpoint, and one opted-in dot. No reply channel or polling."
    },
    {
      title: "Source",
      linkPrefix: "Repository: ",
      linkHref: "https://github.com/amxv/dm-dot",
      linkLabel: "github.com/amxv/dm-dot"
    }
  ]
} as const;

export const docCategories = ["Start", "Reference", "Guides"] as const;

export const primaryNav = [
  { href: "/", label: "Home" },
  { href: "/docs", label: "Docs" },
  { href: siteConfig.repoUrl, label: "GitHub", external: true }
];
