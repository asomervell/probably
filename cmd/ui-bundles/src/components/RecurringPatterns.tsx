import { createRoot } from 'react-dom/client';
import { StatCard } from './shared/StatCard';

function RecurringPatterns() {
  const data = window.openai?.toolOutput || {};
  const metadata = window.openai?.toolResponseMetadata || {};
  const theme = window.openai?.theme || 'light';

  const count = data.count || 0;
  const totalMonthly = data.total_monthly || '$0.00';

  const patterns = metadata.patterns || [];

  return (
    <div style={{ padding: '16px', fontFamily: 'system-ui, sans-serif' }}>
      <h2 style={{ marginTop: 0 }}>Recurring Patterns</h2>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
        <StatCard
          value={count.toString()}
          label="Active Subscriptions"
          trend={null}
        />
        <StatCard
          value={totalMonthly}
          label="Monthly Total"
          trend={null}
        />
      </div>
      {patterns.length > 0 && (
        <div style={{ marginTop: '16px' }}>
          <h3>Subscriptions & Bills</h3>
          {patterns.map((pattern: any, idx: number) => (
            <div key={idx} style={{ 
              marginBottom: '8px', 
              padding: '12px', 
              background: theme === 'dark' ? '#303030' : '#F3F3F3',
              borderRadius: '8px'
            }}>
              <strong>{pattern.entity_name || 'Unknown'}</strong>
              <div style={{ fontSize: '14px', color: theme === 'dark' ? '#AFAFAF' : '#5D5D5D' }}>
                ${((pattern.avg_amount_cents || 0) / 100).toFixed(2)} / {pattern.frequency || 'month'}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

if (typeof window !== 'undefined') {
  const rootElement = document.getElementById('root');
  if (rootElement) {
    const root = createRoot(rootElement);
    root.render(<RecurringPatterns />);
  }
}

export default RecurringPatterns;
