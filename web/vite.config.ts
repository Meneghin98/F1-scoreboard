import { gzipSync } from 'node:zlib';
import { defineConfig, type Plugin } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

// Accanto a ogni file JS/CSS genera la versione compressa .gz: il server Go la
// invia così com'è ai browser che accettano gzip, senza comprimere a runtime.
function precompress(): Plugin {
  return {
    name: 'precompress',
    apply: 'build',
    enforce: 'post',
    generateBundle(_, bundle) {
      for (const [fileName, file] of Object.entries(bundle)) {
        if (!/\.(js|css|html|svg)$/.test(fileName)) continue;
        const source = file.type === 'chunk' ? file.code : file.source;
        if (source.length < 1024) continue; // per file così piccoli non conviene
        this.emitFile({ type: 'asset', fileName: `${fileName}.gz`, source: gzipSync(source, { level: 9 }) });
      }
    },
  };
}

export default defineConfig({
  plugins: [react(), tailwindcss(), precompress()],
  server: {
    // In sviluppo le chiamate /api vanno al server Go (go run .).
    proxy: { '/api': 'http://localhost:3000' },
  },
});
