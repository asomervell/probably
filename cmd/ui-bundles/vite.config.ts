import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// Plugin to inject process polyfill for browser environment
// This ensures process is available before any code runs
const processPolyfillPlugin = () => {
  return {
    name: 'process-polyfill',
    renderChunk(code, chunk) {
      // Inject process polyfill at the top of each chunk
      // This runs immediately when the module loads
      const polyfillCode = `(function() {
  var g = typeof globalThis !== 'undefined' ? globalThis : typeof window !== 'undefined' ? window : typeof global !== 'undefined' ? global : this;
  if (g && typeof g.process === 'undefined') {
    g.process = { env: { NODE_ENV: 'production' } };
  }
})();
`;
      return {
        code: polyfillCode + code,
        map: null,
      };
    },
  };
};

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), processPolyfillPlugin()],
  define: {
    // Replace process.env with browser-safe values
    'process.env.NODE_ENV': JSON.stringify('production'),
    'process.env': '{}',
  },
  build: {
    lib: {
      entry: {
        'spending-summary': './src/components/SpendingSummary.tsx',
        'account-balances': './src/components/AccountBalances.tsx',
        'ask-question': './src/components/AskQuestion.tsx',
        'spending-trends': './src/components/SpendingTrends.tsx',
        'recurring-patterns': './src/components/RecurringPatterns.tsx',
        'search-transactions': './src/components/SearchTransactions.tsx',
        'financial-overview': './src/components/FinancialOverview.tsx',
      },
      formats: ['es'],
      fileName: (format, entryName) => `${entryName}.js`,
    },
    rollupOptions: {
      // Don't externalize React - bundle it so it works in ChatGPT sandbox
      // ChatGPT sandbox may not provide React globally
      external: ['@openai/apps-sdk-ui'],
      output: {
        // No banner needed - polyfill is injected via renderChunk
      },
    },
  },
});
