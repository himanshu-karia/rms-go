import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
export default defineConfig({
    server: {
        port: 5665,
        proxy: {
            '/api': {
                target: 'https://localhost:7443',
                changeOrigin: true,
                secure: false,
            },
            '/mqtt': {
                target: 'https://localhost:7443',
                changeOrigin: true,
                secure: false,
                ws: true,
            },
        },
    },
    preview: {
        port: 5665,
    },
    plugins: [react()],
    resolve: {
        extensions: ['.ts', '.tsx', '.js', '.jsx', '.json'],
    },
});
