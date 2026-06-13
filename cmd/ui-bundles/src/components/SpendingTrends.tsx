import { createRoot } from 'react-dom/client';

function SpendingTrends() {
  const data = window.openai?.toolOutput || {};
  const theme = window.openai?.theme || 'light';

  const trend = data.trend || 'stable';
  const change = data.change || 0;
  const period = data.period || 'month';

  return (
    <div style={{ padding: '16px', fontFamily: 'system-ui, sans-serif' }}>
      <h2 style={{ marginTop: 0 }}>Spending Trends ({period})</h2>
      <div style={{ 
        padding: '12px', 
        background: theme === 'dark' ? '#303030' : '#F3F3F3',
        borderRadius: '8px'
      }}>
        <div>Trend: <strong>{trend}</strong></div>
        <div>Change: {change > 0 ? '+' : ''}{change}%</div>
      </div>
      <p style={{ fontSize: '14px', color: theme === 'dark' ? '#AFAFAF' : '#5D5D5D' }}>
        Chart visualization will be added when Apps SDK UI Chart components are available.
      </p>
    </div>
  );
}

if (typeof window !== 'undefined') {
  const rootElement = document.getElementById('root');
  if (rootElement) {
    const root = createRoot(rootElement);
    root.render(<SpendingTrends />);
  }
}

export default SpendingTrends;
