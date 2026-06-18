import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const apiTarget = process.env.VOLA_DEV_API_TARGET || 'http://localhost:8080'
const gatewayProxy = () => ({ target: apiTarget, changeOrigin: true })
const exactGatewayProxy = (path: string) => ({
  ...gatewayProxy(),
  bypass: (req: { method?: string; url?: string }) => {
    const url = req.url || ''
    if (url === path || url.startsWith(`${path}?`) || url.startsWith(`${path}/`)) return undefined
    if (req.method === 'GET' || req.method === 'HEAD') return '/index.html'
    return undefined
  },
})

function manualChunks(id: string) {
  const normalized = id.replace(/\\/g, '/')
  if (!normalized.includes('/node_modules/')) return undefined
  if (
    normalized.includes('/react/') ||
    normalized.includes('/react-dom/') ||
    normalized.includes('/react-router') ||
    normalized.includes('/scheduler/')
  ) {
    return 'vendor-react'
  }
  if (
    normalized.includes('/@uiw/react-codemirror/') ||
    normalized.includes('/@uiw/codemirror-extensions-basic-setup/') ||
    normalized.includes('/codemirror/') ||
    normalized.includes('/@codemirror/') ||
    normalized.includes('/@lezer/') ||
    normalized.includes('/style-mod/') ||
    normalized.includes('/w3c-keyname/') ||
    normalized.includes('/crelt/') ||
    normalized.includes('/@marijn/find-cluster-break/')
  ) {
    return 'vendor-editor'
  }
  if (
    normalized.includes('/@uiw/react-markdown-preview/') ||
    normalized.includes('/react-markdown/') ||
    normalized.includes('/@uiw/copy-to-clipboard/') ||
    normalized.includes('/rehype-') ||
    normalized.includes('/remark-') ||
    normalized.includes('/micromark') ||
    normalized.includes('/mdast-util-') ||
    normalized.includes('/hast-util-') ||
    normalized.includes('/unist-util-') ||
    normalized.includes('/unified/') ||
    normalized.includes('/vfile/') ||
    normalized.includes('/vfile-message/') ||
    normalized.includes('/property-information/') ||
    normalized.includes('/entities/') ||
    normalized.includes('/parse5/') ||
    normalized.includes('/web-namespaces/') ||
    normalized.includes('/html-void-elements/') ||
    normalized.includes('/html-url-attributes/') ||
    normalized.includes('/comma-separated-tokens/') ||
    normalized.includes('/space-separated-tokens/') ||
    normalized.includes('/style-to-js/') ||
    normalized.includes('/style-to-object/') ||
    normalized.includes('/inline-style-parser/') ||
    normalized.includes('/css-selector-parser/') ||
    normalized.includes('/nth-check/') ||
    normalized.includes('/bcp-47-match/') ||
    normalized.includes('/direction/') ||
    normalized.includes('/decode-named-character-reference/') ||
    normalized.includes('/character-') ||
    normalized.includes('/markdown-table/') ||
    normalized.includes('/longest-streak/') ||
    normalized.includes('/bail/') ||
    normalized.includes('/trough/') ||
    normalized.includes('/is-plain-obj/')
  ) {
    return 'vendor-markdown'
  }
  if (normalized.includes('/@tauri-apps/')) {
    return 'vendor-tauri'
  }
  return undefined
}

export default defineConfig({
  plugins: [react()],
  build: {
    chunkSizeWarningLimit: 700,
    rollupOptions: {
      output: {
        manualChunks,
      },
    },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': gatewayProxy(),
      '/agent': gatewayProxy(),
      '/mcp': exactGatewayProxy('/mcp'),
      '/gpt': gatewayProxy(),
      '/stripe': gatewayProxy(),
      '/.well-known': gatewayProxy(),
      '/oauth/authorize': {
        ...gatewayProxy(),
        bypass: (req) => {
          if (req.method === 'GET' || req.method === 'HEAD') {
            return '/index.html'
          }
        }
      },
      '/oauth': gatewayProxy()
    }
  }
})
