import React, { useState } from 'react';
import ReactDOM from 'react-dom/client';
import { useRegisterSW } from 'virtual:pwa-register/react';
import App from './App';
import './index.css';

function UpdateToast() {
  const {
    needRefresh: [needRefresh, setNeedRefresh],
    updateServiceWorker,
  } = useRegisterSW();
  const [dismissed, setDismissed] = useState(false);

  if (!needRefresh || dismissed) return null;

  return (
    <div style={{
      position: 'fixed', bottom: 16, left: '50%', transform: 'translateX(-50%)',
      zIndex: 9999, display: 'flex', alignItems: 'center', gap: 12,
      background: '#1e293b', color: '#fff', borderRadius: 12,
      padding: '10px 16px', fontSize: 14, boxShadow: '0 4px 16px rgba(0,0,0,.25)',
      whiteSpace: 'nowrap',
    }}>
      <span>A new version of TPT Admin is available.</span>
      <button
        onClick={() => { void updateServiceWorker(true); }}
        style={{ background: '#0d9488', color: '#fff', border: 'none', borderRadius: 6, padding: '4px 12px', cursor: 'pointer', fontSize: 12, fontWeight: 600 }}
      >
        Reload to update
      </button>
      <button
        onClick={() => { setNeedRefresh(false); setDismissed(true); }}
        style={{ background: 'none', border: 'none', color: '#94a3b8', cursor: 'pointer', fontSize: 16, padding: '0 2px' }}
        aria-label="Dismiss"
      >
        ✕
      </button>
    </div>
  );
}

function Root() {
  return (
    <React.StrictMode>
      <App />
      <UpdateToast />
    </React.StrictMode>
  );
}

ReactDOM.createRoot(document.getElementById('root')!).render(<Root />);
