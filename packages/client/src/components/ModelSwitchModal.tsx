import { useEffect, useState } from "react";
import { api } from "../api/client";
import { Modal } from "./ui/Modal";

interface ModelSwitchModalProps {
  processId: string;
  currentModel?: string;
  onModelChanged: (model: string) => void;
  onClose: () => void;
}

interface ModelOption {
  id: string;
  name: string;
  description?: string;
}

export function ModelSwitchModal({
  processId,
  currentModel,
  onModelChanged,
  onClose,
}: ModelSwitchModalProps) {
  const [models, setModels] = useState<ModelOption[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [switching, setSwitching] = useState(false);

  useEffect(() => {
    api
      .getProcessModels(processId)
      .then((res) => setModels(res.models))
      .catch((err) => setError(err.message || "Failed to load models"))
      .finally(() => setLoading(false));
  }, [processId]);

  const handleSelect = async (modelId: string) => {
    if (switching) return;
    setSwitching(true);
    setError(null);
    try {
      await api.setProcessModel(processId, modelId);
      onModelChanged(modelId);
      onClose();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to switch model");
      setSwitching(false);
    }
  };

  return (
    <Modal title="Switch Model" onClose={onClose}>
      <div className="model-switch-content">
        {loading && (
          <div className="model-switch-loading">Loading models...</div>
        )}
        {error && <div className="model-switch-error">{error}</div>}
        {!loading && !error && models.length === 0 && (
          <div className="model-switch-loading">No models available</div>
        )}
        {!loading && models.length > 0 && (
          <div className="model-switch-list">
            {models.map((model) => {
              const isCurrent = currentModel
                ? currentModel.includes(model.id) ||
                  model.id.includes(currentModel)
                : false;
              return (
                <button
                  key={model.id}
                  type="button"
                  className={`model-switch-item ${isCurrent ? "current" : ""}`}
                  onClick={() => handleSelect(model.id)}
                  disabled={switching}
                >
                  <span className="model-switch-name">{model.name}</span>
                  {model.description && (
                    <span className="model-switch-description">
                      {model.description}
                    </span>
                  )}
                  {isCurrent && (
                    <span className="model-switch-badge">Current</span>
                  )}
                </button>
              );
            })}
          </div>
        )}
      </div>
    </Modal>
  );
}
