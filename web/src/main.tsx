import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './App';
import { ConfirmProvider } from './lib/confirm';
import { ToastProvider } from './lib/toast';
import './index.css';

const root = document.getElementById('root');
if (!root) throw new Error('manca l\'elemento #root in index.html');

createRoot(root).render(
  <StrictMode>
    <ToastProvider>
      <ConfirmProvider>
        <App />
      </ConfirmProvider>
    </ToastProvider>
  </StrictMode>,
);
