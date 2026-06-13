import { createRoot } from 'react-dom/client';
import { StatCard } from './shared/StatCard';

function SpendingSummary() {
  const data = window.openai?.toolOutput || {};
  const theme = window.openai?.theme || 'light';

  const total = data.total || '0';
  const period = data.period || 'month';
  const byCategory = data.by_category || [];

  // Show error if no data
  if (!data.total && byCategory.length === 0) {
    return (
      <div style={{ padding: '16px', fontFamily: 'system-ui, sans-serif', color: '#ff6b6b' }}>
        <h2 style={{ marginTop: 0 }}>Spending Summary</h2>
        <p>No data available.</p>
      </div>
    );
  }

  return (
    <div style={{ padding: '16px', fontFamily: 'system-ui, sans-serif' }}>
      <h2 style={{ marginTop: 0 }}>Spending Summary ({period})</h2>
      <StatCard
        value={total}
        label="Total Spending"
        trend={null}
      />
      {byCategory.length > 0 && (
        <div style={{ marginTop: '16px' }}>
          <h3>By Category</h3>
          {byCategory.map((cat: any, idx: number) => (
            <div key={idx} style={{ marginBottom: '8px', padding: '8px', background: theme === 'dark' ? '#303030' : '#F3F3F3' }}>
              <strong>{cat.category}</strong>: {cat.amount}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// Initialize widget when DOM is ready
if (typeof window !== 'undefined') {
  // Wait for DOM to be ready
  const initWidget = () => {
    const rootElement = document.getElementById('root');
    if (rootElement) {
      try {
        const root = createRoot(rootElement);
        root.render(<SpendingSummary />);
      } catch (error) {
        console.error('[SpendingSummary] Error rendering:', error);
        rootElement.innerHTML = `<div style="padding: 16px; color: #ff6b6b;">
          <h2>Error Loading Widget</h2>
          <p>${error instanceof Error ? error.message : String(error)}</p>
        </div>`;
      }
    } else {
      console.error('[SpendingSummary] Root element not found');
    }
  };

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initWidget);
  } else {
    initWidget();
  }
}

export default SpendingSummary;
