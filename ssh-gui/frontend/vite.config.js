import {defineConfig} from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
<<<<<<< HEAD
    host: true,
  }
=======
    host: '127.0.0.1',
  },
>>>>>>> ca8e195 (aplica algunos fixes)
})
