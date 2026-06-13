import { createRoot } from 'react-dom/client';
import { StatCard } from './shared/StatCard';

function FinancialOverview() {
  const data = window.openai?.toolOutput || {};

  const netWorth = data.net_worth || 0;
  const totalAssets = data.total_assets || 0;
  const totalLiabilities = data.total_liabilities || 0;

  const formatCurrency = (cents: number) => {
    return `$${(cents / 100).toFixed(2)}`;
  };

  return (
    <div style={{ padding: '16px', fontFamily: 'system-ui, sans-serif' }}>
      <h2 style={{ marginTop: 0 }}>Financial Overview</h2>
      <StatCard
        value={formatCurrency(netWorth)}
        label="Net Worth"
        trend={null}
      />
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px', marginTop: '16px' }}>
        <StatCard
          value={formatCurrency(totalAssets)}
          label="Total Assets"
          trend={null}
        />
        <StatCard
          value={formatCurrency(totalLiabilities)}
          label="Total Liabilities"
          trend={null}
        />
      </div>
    </div>
  );
}

if (typeof window !== 'undefined') {
  const rootElement = document.getElementById('root');
  if (rootElement) {
    const root = createRoot(rootElement);
    root.render(<FinancialOverview />);
  }
}

export default FinancialOverview;
