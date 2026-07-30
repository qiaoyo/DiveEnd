import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useAppStore } from '../../stores/appStore';
import { defaultConfig } from '../../types';

const backendMocks = vi.hoisted(() => ({
  getSecretPrefill: vi.fn(),
  getLLMUsage: vi.fn(),
  saveConfig: vi.fn(),
}));

vi.mock('../../lib/backend', () => backendMocks);

import { SettingsPanel } from './SettingsPanel';

function cloneDefaultConfig() {
  return JSON.parse(JSON.stringify(defaultConfig)) as typeof defaultConfig;
}

describe('SettingsPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();

    const config = cloneDefaultConfig();
    config.dataPath = '/Users/example/DiveEndData';
    config.llm.hasApiKey = false;
    config.weakLLM.hasApiKey = false;
    config.baiduCloud.hasToken = false;

    backendMocks.getSecretPrefill.mockResolvedValue({
      strongLLMApiKey: '',
      hasStrongLLMApiKey: false,
      weakLLMApiKey: '',
      hasWeakLLMApiKey: false,
      baiduToken: '',
      hasBaiduToken: false,
    });
    backendMocks.getLLMUsage.mockResolvedValue({
      date: '2026-07-30',
      usedTokens: 1234,
      limit: 100_000_000,
      remaining: 99_998_766,
    });
    backendMocks.saveConfig.mockImplementation(async (incomingConfig) => ({
      config: {
        ...incomingConfig,
        llm: {
          ...incomingConfig.llm,
          apiKey: '',
          hasApiKey: incomingConfig.llm.apiKey.trim().length > 0 || incomingConfig.llm.hasApiKey,
        },
      },
      restartRequired: false,
    }));

    useAppStore.setState({
      config,
      isSavingConfig: false,
      leftPanelCollapsed: false,
      error: null,
    });
  });

  it('keeps unsaved workspace changes when editing secret fields', async () => {
    render(<SettingsPanel />);

    fireEvent.change(screen.getByDisplayValue('/Users/example/DiveEndData'), {
      target: { value: '/Users/example/NewDiveEndData' },
    });
    fireEvent.click(screen.getByRole('tab', { name: '凭据' }));

    const strongKeyInput = screen.getAllByPlaceholderText('sk-...')[0];
    fireEvent.change(strongKeyInput, { target: { value: 'sk-new-key' } });

    await waitFor(() => expect(strongKeyInput).toHaveValue('sk-new-key'));
    fireEvent.click(screen.getByRole('button', { name: '保存配置' }));

    await waitFor(() => expect(backendMocks.saveConfig).toHaveBeenCalledTimes(1));
    const payload = backendMocks.saveConfig.mock.calls[0][0];
    expect(payload.dataPath).toBe('/Users/example/NewDiveEndData');
    expect(payload.llm.apiKey).toBe('sk-new-key');
  });

  it('redacts sensitive values in local save status messages', async () => {
    backendMocks.saveConfig.mockRejectedValueOnce(
      new Error('save failed api_key=sk-settings-secret and access_token=settings-token-secret')
    );

    render(<SettingsPanel />);
    fireEvent.click(screen.getByRole('button', { name: '保存配置' }));

    await waitFor(() => {
      expect(document.body.textContent).toContain('api_key=[redacted]');
      expect(document.body.textContent).toContain('access_token=[redacted]');
    });
    expect(document.body.textContent).not.toContain('sk-settings-secret');
    expect(document.body.textContent).not.toContain('settings-token-secret');
  });
});
