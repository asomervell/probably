type Trend = {
  direction: 'up' | 'down';
  value: string;
};

type StatCardProps = {
  value: React.ReactNode;
  label: string;
  trend?: Trend | null;
};

const cardStyle: React.CSSProperties = {
  borderRadius: '12px',
  padding: '16px',
  background: 'rgba(255, 255, 255, 0.05)',
  border: '1px solid rgba(255, 255, 255, 0.08)',
  boxShadow: '0 4px 16px rgba(0, 0, 0, 0.08)',
  display: 'flex',
  flexDirection: 'column',
  gap: '8px',
};

const labelStyle: React.CSSProperties = {
  fontSize: '14px',
  color: '#7a7a7a',
  textTransform: 'uppercase',
  letterSpacing: '0.08em',
};

const valueStyle: React.CSSProperties = {
  fontSize: '24px',
  fontWeight: 600,
  color: '#f5f5f5',
};

const trendStyle: React.CSSProperties = {
  fontSize: '13px',
  fontWeight: 500,
  display: 'inline-flex',
  alignItems: 'center',
  gap: '4px',
};

const arrow = (direction: 'up' | 'down') => (direction === 'up' ? '▲' : '▼');
const trendColor = (direction: 'up' | 'down') =>
  direction === 'up' ? '#4CAF50' : '#EF5350';

export function StatCard({ value, label, trend }: StatCardProps) {
  return (
    <div style={cardStyle}>
      <span style={labelStyle}>{label}</span>
      <span style={valueStyle}>{value}</span>
      {trend && (
        <span style={{ ...trendStyle, color: trendColor(trend.direction) }}>
          {arrow(trend.direction)}
          {trend.value}
        </span>
      )}
    </div>
  );
}
