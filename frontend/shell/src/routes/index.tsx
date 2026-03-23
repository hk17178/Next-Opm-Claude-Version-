/**
 * 全局路由配置
 *
 * 包含：
 * - 默认跳转：/ → /log/search
 * - 错误页面：404、403、500、503 维护页
 * - 微前端容器：匹配其余所有路径
 */
import React from 'react';
import { Navigate, type RouteObject } from 'react-router-dom';
import NotFound from '../pages/error/NotFound';
import Forbidden from '../pages/error/Forbidden';
import ServerError from '../pages/error/ServerError';
import Maintenance from '../pages/error/Maintenance';

const routes: RouteObject[] = [
  {
    path: '/',
    element: <Navigate to="/log/search" replace />,
  },
  {
    /** 403 无权限页面 */
    path: '/403',
    element: <Forbidden />,
  },
  {
    /** 404 页面不存在 */
    path: '/404',
    element: <NotFound />,
  },
  {
    /** 500 系统异常页面 */
    path: '/500',
    element: <ServerError />,
  },
  {
    /** 503 系统维护中页面 */
    path: '/maintenance',
    element: <Maintenance />,
  },
  {
    /** 微前端子应用容器，匹配其余所有路径 */
    path: '*',
    element: <div id="micro-app-container" />,
  },
];

export default routes;
