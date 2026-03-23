import React, { useEffect } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { start, registerMicroApps } from 'qiankun';
import BasicLayout from './layouts/BasicLayout';
import { microApps } from './config/apps';

const App: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    registerMicroApps(
      microApps.map((app) => ({
        ...app,
        props: { navigate },
      })),
    );

    start({
      sandbox: { experimentalStyleIsolation: true },
      prefetch: 'all',
    });
  }, [navigate]);

  // Redirect root path to home
  useEffect(() => {
    if (location.pathname === '/') {
      navigate('/home', { replace: true });
    }
  }, [location.pathname, navigate]);

  return <BasicLayout />;
};

export default App;
