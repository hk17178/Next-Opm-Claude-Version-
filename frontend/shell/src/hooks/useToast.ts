/**
 * useToast - 全局 Toast 通知 Hook
 *
 * 封装 Ant Design 的 message 和 notification API，提供统一的通知调用方式。
 * 不同类型的通知有不同的默认行为：
 * - 成功通知：3 秒后自动消失
 * - 警告通知：5 秒后自动消失
 * - 错误通知：不自动消失，需手动关闭
 * - 信息通知：3 秒后自动消失
 *
 * 使用方式：
 * ```tsx
 * const toast = useToast();
 * toast.showSuccess('操作成功');
 * toast.showError('操作失败，请重试');
 * ```
 */
import { message, notification } from 'antd';

/**
 * Toast 通知管理器
 * 提供 showSuccess / showWarning / showError / showInfo 四种通知方法
 */
export function useToast() {
  /**
   * 显示成功通知
   * @param msg - 提示文案
   * @param duration - 持续时间（秒），默认 3 秒
   */
  const showSuccess = (msg: string, duration = 3) => {
    message.success({
      content: msg,
      duration,
    });
  };

  /**
   * 显示警告通知
   * @param msg - 提示文案
   * @param duration - 持续时间（秒），默认 5 秒
   */
  const showWarning = (msg: string, duration = 5) => {
    message.warning({
      content: msg,
      duration,
    });
  };

  /**
   * 显示错误通知
   * 错误通知默认不自动消失，需用户手动关闭
   * @param msg - 提示文案
   * @param duration - 持续时间（秒），默认 0（不自动关闭）
   */
  const showError = (msg: string, duration = 0) => {
    notification.error({
      message: '操作失败',
      description: msg,
      duration,
      placement: 'topRight',
    });
  };

  /**
   * 显示信息通知
   * @param msg - 提示文案
   * @param duration - 持续时间（秒），默认 3 秒
   */
  const showInfo = (msg: string, duration = 3) => {
    message.info({
      content: msg,
      duration,
    });
  };

  return {
    showSuccess,
    showWarning,
    showError,
    showInfo,
  };
}

export default useToast;
