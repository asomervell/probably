import { createRoot } from 'react-dom/client';

function SearchTransactions() {
  const data = window.openai?.toolOutput || {};
  const metadata = window.openai?.toolResponseMetadata || {};
  const theme = window.openai?.theme || 'light';

  const count = data.count || 0;
  const query = data.query || '';

  const transactions = metadata.transactions || [];

  return (
    <div style={{ padding: '16px', fontFamily: 'system-ui, sans-serif' }}>
      <h2 style={{ marginTop: 0 }}>Transaction Search</h2>
      {query && <p>Query: <strong>{query}</strong></p>}
      <p>Found: <strong>{count}</strong> transactions</p>
      {transactions.length > 0 && (
        <div style={{ marginTop: '16px' }}>
          {transactions.slice(0, 10).map((txn: any, idx: number) => (
            <div key={idx} style={{ 
              marginBottom: '8px', 
              padding: '12px', 
              background: theme === 'dark' ? '#303030' : '#F3F3F3',
              borderRadius: '8px'
            }}>
              <div><strong>{txn.description || 'No description'}</strong></div>
              <div style={{ fontSize: '14px', color: theme === 'dark' ? '#AFAFAF' : '#5D5D5D' }}>
                {txn.date || 'No date'}
              </div>
            </div>
          ))}
          {transactions.length > 10 && (
            <p style={{ fontSize: '14px', color: theme === 'dark' ? '#AFAFAF' : '#5D5D5D' }}>
              ... and {transactions.length - 10} more
            </p>
          )}
        </div>
      )}
    </div>
  );
}

if (typeof window !== 'undefined') {
  const rootElement = document.getElementById('root');
  if (rootElement) {
    const root = createRoot(rootElement);
    root.render(<SearchTransactions />);
  }
}

export default SearchTransactions;
