import { defineConfig } from 'vitepress'

export default defineConfig({
  ignoreDeadLinks: true,
  title: "Cosmopilot",
  description: "Cosmopilot Documentation",
  head: [
    ['link', { rel: "icon", type: "image/png", sizes: "96x96", href: "/favicon-96x96.png" }],
    ['link', { rel: "icon", type: "image/svg+xml", href: "/favicon.svg" }],
    ['link', { rel: "shortcut icon", href: "/favicon.ico" }],
    ['link', { rel: "apple-touch-icon", sizes: "180x180", href: "/apple-touch-icon.png" }],
    ['meta', { name: "apple-mobile-web-app-title", content: "Cosmopilot" }],
    ['link', { rel: "manifest", href: "/site.webmanifest" }],
  ],
  themeConfig: {
    logo: {
      src: '/logo.png',
      alt: 'Cosmopilot'
    },
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Getting Started', link: '/01-getting-started/01-prerequisites' },
      { text: 'Usage', link: '/02-usage/01-deploy-node' }
    ],
    sidebar: [
      {
        text: 'Getting Started',
        base: '/01-getting-started/',
        items: [
          { text: 'Prerequisites', link: '01-prerequisites' },
          { text: 'Installation', link: '02-installation' },
          { text: 'Configuration', link: '03-configuration' }
        ]
      },
      {
        text: 'Usage',
        base: '/02-usage/',
        items: [
          { text: 'Deploying a Node', link: '01-deploy-node' },
          { text: 'Deploying a Node Set', link: '02-deploy-node-set' },
          { text: 'Genesis Download', link: '03-genesis' },
          { text: 'Node Configurations', link: '04-node-config' },
          { text: 'Persistence and Backups', link: '05-persistence-and-backup' },
          { text: 'Restore from Snapshot', link: '06-restoring-from-snapshot' },
          { text: 'Exposing Endpoints', link: '07-exposing-endpoints' },
          { text: 'Upgrades', link: '08-upgrades' },
          { text: 'Validator', link: '09-validator' },
          { text: 'Initializing new Network', link: '10-initializing-new-network' },
          { text: 'Using TmKMS', link: '11-tmkms' },
          { text: 'Using CosmoGuard', link: '12-cosmoguard' },
        ]
      },
      {
        text: 'Reference',
        base: '/03-reference/',
        items: [
          { text: 'Custom Resource Definitions', link: 'crds/crds' },
        ]
      },
      {
        text: 'Examples',
        items: [
          {
            text: 'Nibiru',
            base: '/04-examples/nibiru/',
            items: [
              { text: 'Validator + Fullnode', link: 'devnet-one-fullnode' },
              { text: 'Validator with TmKMS', link: 'validator-tmkms' },
              { text: 'Multi-Validator Devnet', link: 'multi-validator-devnet' },
            ]
          },
          {
            text: 'Osmosis',
            base: '/04-examples/osmosis/',
            items: [
              { text: 'Validator + Fullnode', link: 'devnet-one-fullnode' },
              { text: 'Multi-Validator Devnet', link: 'multi-validator-devnet' },
            ]
          },
          {
            text: 'Cosmos',
            base: '/04-examples/cosmos/',
            items: [
              { text: 'Validator + Fullnode', link: 'devnet-one-fullnode' },
              { text: 'Multi-Validator Devnet', link: 'multi-validator-devnet' },
            ]
          },
          {
            text: 'Sei',
            base: '/04-examples/sei/',
            items: [
              { text: 'Validator + Fullnode', link: 'devnet-one-fullnode' },
              { text: 'Multi-Validator Devnet', link: 'multi-validator-devnet' },
            ]
          },
        ]
      }
    ],
    socialLinks: [
      { icon: 'github', link: 'https://github.com/NibiruChain/cosmopilot' }
    ],
    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright © 2024-present NIBIRU'
    }
  }
})
