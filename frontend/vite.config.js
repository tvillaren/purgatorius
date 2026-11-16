// Config Vite : front Vue 3, compile dans dist/ puis embarque par main.go
// (//go:embed all:frontend/dist).
import { writeFileSync } from 'node:fs';
import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

// Vite vide dist/ a chaque build, gitkeep compris. Or ce fichier doit rester
// suivi par git : sur un clone neuf, //go:embed echoue si dist/ est vide, et la
// CI lance go test avant de compiler le front.
const garderGitkeep = {
  name: 'garder-gitkeep',
  closeBundle() {
    writeFileSync(new URL('./dist/gitkeep', import.meta.url), '');
  },
};

export default defineConfig({
  plugins: [vue(), garderGitkeep],
});
