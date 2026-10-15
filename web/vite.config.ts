import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    // In sviluppo le chiamate /api vanno al server Go (go run .).
    proxy: { '/api': 'http://localhost:3000' },
  },
});
