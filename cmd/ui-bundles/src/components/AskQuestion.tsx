import { createRoot } from 'react-dom/client';

function AskQuestion() {
  const data = window.openai?.toolOutput || {};
  const theme = window.openai?.theme || 'light';

  const answer = data.answer || 'No answer available';
  const question = data.question || '';

  return (
    <div style={{ padding: '16px', fontFamily: 'system-ui, sans-serif' }}>
      {question && <h3 style={{ marginTop: 0 }}>Q: {question}</h3>}
      <div style={{ 
        padding: '12px', 
        background: theme === 'dark' ? '#303030' : '#F3F3F3',
        borderRadius: '8px',
        whiteSpace: 'pre-wrap'
      }}>
        {answer}
      </div>
    </div>
  );
}

if (typeof window !== 'undefined') {
  const rootElement = document.getElementById('root');
  if (rootElement) {
    const root = createRoot(rootElement);
    root.render(<AskQuestion />);
  }
}

export default AskQuestion;
