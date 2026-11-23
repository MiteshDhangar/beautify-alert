import React from 'react';
import { Navigate, Route, Routes } from 'react-router-dom';
import { AppRootProps } from '@grafana/data';
import { ROUTES } from '../../constants';

const TemplatesPage = React.lazy(() => import('../../pages/Templates/TemplatesPage'));
const HistoryPage = React.lazy(() => import('../../pages/History/HistoryPage'));
const SetupPage = React.lazy(() => import('../../pages/Setup/SetupPage'));

function App(_props: AppRootProps) {
  return (
    <Routes>
      <Route path={ROUTES.Templates} element={<TemplatesPage />} />
      <Route path={ROUTES.History} element={<HistoryPage />} />
      <Route path={ROUTES.Setup} element={<SetupPage />} />
      <Route path="*" element={<Navigate to={ROUTES.Templates} replace />} />
    </Routes>
  );
}

export default App;
