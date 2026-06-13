import { createRoot } from 'react-dom/client';
import { StatCard } from './shared/StatCard';

function AccountBalances() {
  const data = window.openai?.toolOutput || {};
  const metadata = window.openai?.toolResponseMetadata || {};

  const rawValues = metadata.raw_values || {};

  const getRawCents = (field: string): number => {
    if (typeof rawValues[field] === 'number') {
      return rawValues[field];
    }
    if (typeof data[field] === 'number') {
      return data[field];
    }
    const fallbackKey = field.replace('_cents', '');
    if (typeof data[fallbackKey] === 'number') {
      return data[fallbackKey];
    }
    return 0;
  };

  const formatCurrency = (cents: number = 0) => {
    return `$${(cents / 100).toFixed(2)}`;
  };

  const getDisplayValue = (display: unknown, cents: number) => {
    if (typeof display === 'string' && display.trim().length > 0) {
      return display;
    }
    return formatCurrency(cents);
  };

  const netWorthCents = getRawCents('net_worth_cents');
  const totalAssetsCents = getRawCents('total_assets_cents');
  const totalLiabilitiesCents = getRawCents('total_liabilities_cents');

  const netWorthDisplay = getDisplayValue(data.net_worth, netWorthCents);
  const totalAssetsDisplay = getDisplayValue(data.total_assets, totalAssetsCents);
  const totalLiabilitiesDisplay = getDisplayValue(data.total_liabilities, totalLiabilitiesCents);

  return (
    <div style={{ padding: '16px', fontFamily: 'system-ui, sans-serif' }}>
      <h2 style={{ marginTop: 0 }}>Account Balances</h2>
      <StatCard
        value={netWorthDisplay}
        label="Net Worth"
        trend={null}
      />
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px', marginTop: '16px' }}>
        <StatCard
          value={totalAssetsDisplay}
          label="Total Assets"
          trend={null}
        />
        <StatCard
          value={totalLiabilitiesDisplay}
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
    root.render(<AccountBalances />);
  }
}

export default AccountBalances;
