import { useState } from "react";
import { Plus, X } from "lucide-react";
import { ApiError, createContainer, type CreateContainerPort } from "../api/client";
import { useToast } from "./ui/toastContext";
import { Button } from "./ui/Button";
import { FormError } from "./ui/Form";
import { Modal } from "./ui/Modal";

interface CreateContainerModalProps {
  onClose: () => void;
  onCreated: () => void;
}

const inputClass = "field";

export function CreateContainerModal({ onClose, onCreated }: CreateContainerModalProps) {
  const [image, setImage] = useState("");
  const [name, setName] = useState("");
  const [restartPolicy, setRestartPolicy] = useState<"no" | "always" | "on-failure" | "unless-stopped">("no");
  const [envLines, setEnvLines] = useState("");
  const [ports, setPorts] = useState<CreateContainerPort[]>([]);
  const [bindLines, setBindLines] = useState("");
  const [command, setCommand] = useState("");
  const [memoryMb, setMemoryMb] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const toast = useToast();

  const addPort = () => setPorts((prev) => [...prev, { hostPort: "", containerPort: "", protocol: "tcp" }]);
  const updatePort = (i: number, patch: Partial<CreateContainerPort>) =>
    setPorts((prev) => prev.map((p, idx) => (idx === i ? { ...p, ...patch } : p)));
  const removePort = (i: number) => setPorts((prev) => prev.filter((_, idx) => idx !== i));

  const handleCreate = async () => {
    setBusy(true);
    setError(undefined);
    try {
      const { id } = await createContainer({
        image: image.trim(),
        name: name.trim(),
        restartPolicy,
        env: envLines
          .split("\n")
          .map((l) => l.trim())
          .filter(Boolean),
        binds: bindLines
          .split("\n")
          .map((l) => l.trim())
          .filter(Boolean),
        ports: ports.filter((p) => p.hostPort && p.containerPort),
        ...(command.trim() ? { command: command.trim() } : {}),
        ...(memoryMb.trim() ? { memoryBytes: Number(memoryMb) * 1024 * 1024 } : {}),
      });
      toast.push("success", `Created and started container ${id.slice(0, 12)}.`);
      onCreated();
      onClose();
    } catch (err) {
      // 502 with an id means the daemon created the container but could not
      // start it. Retrying would create a second one, so treat it as a
      // partial success: refresh the list, say what happened, and close.
      const createdId =
        err instanceof ApiError && err.status === 502
          ? (err.body as { id?: string } | null)?.id
          : undefined;
      if (createdId) {
        toast.push("error", `Container ${createdId.slice(0, 12)} was created but did not start. Start it from the list.`);
        onCreated();
        onClose();
        return;
      }
      setError(err instanceof ApiError ? err.message : "Failed to create container.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal
      title="New container"
      size="lg"
      onClose={onClose}
      footer={
        <>
          <Button variant="ghost" onClick={onClose} disabled={busy}>
            Cancel
          </Button>
          <Button variant="primary" onClick={handleCreate} loading={busy} disabled={!image.trim()}>
            Create &amp; start
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <div>
          <label className="mb-1 block text-xs font-medium text-ink-muted" htmlFor="cc-image">
            Image
          </label>
          <input
            id="cc-image"
            type="text"
            autoFocus
            placeholder="nginx:latest"
            value={image}
            disabled={busy}
            onChange={(e) => setImage(e.target.value)}
            className={`${inputClass} font-mono`}
          />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="mb-1 block text-xs font-medium text-ink-muted" htmlFor="cc-name">
              Name (optional)
            </label>
            <input
              id="cc-name"
              type="text"
              placeholder="my-app"
              value={name}
              disabled={busy}
              onChange={(e) => setName(e.target.value)}
              className={inputClass}
            />
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-ink-muted" htmlFor="cc-restart">
              Restart policy
            </label>
            <select
              id="cc-restart"
              value={restartPolicy}
              disabled={busy}
              onChange={(e) => setRestartPolicy(e.target.value as typeof restartPolicy)}
              className={inputClass}
            >
              <option value="no">No</option>
              <option value="always">Always</option>
              <option value="on-failure">On failure</option>
              <option value="unless-stopped">Unless stopped</option>
            </select>
          </div>
        </div>

        <div>
          <div className="mb-1 flex items-center justify-between">
            <label className="text-xs font-medium text-ink-muted">Port mappings</label>
            <button
              type="button"
              onClick={addPort}
              disabled={busy}
              className="flex items-center gap-1 text-xs text-accent hover:opacity-80"
            >
              <Plus className="h-3 w-3" /> Add
            </button>
          </div>
          <div className="space-y-2">
            {ports.map((p, i) => (
              <div key={i} className="flex items-center gap-2">
                <input
                  type="text"
                  placeholder="host"
                  value={p.hostPort}
                  disabled={busy}
                  onChange={(e) => updatePort(i, { hostPort: e.target.value })}
                  className={`${inputClass} w-20`}
                />
                <span className="text-ink-faint">:</span>
                <input
                  type="text"
                  placeholder="container"
                  value={p.containerPort}
                  disabled={busy}
                  onChange={(e) => updatePort(i, { containerPort: e.target.value })}
                  className={`${inputClass} w-24`}
                />
                <select
                  value={p.protocol}
                  disabled={busy}
                  onChange={(e) => updatePort(i, { protocol: e.target.value as "tcp" | "udp" })}
                  className={`${inputClass} w-20`}
                >
                  <option value="tcp">tcp</option>
                  <option value="udp">udp</option>
                </select>
                <button
                  type="button"
                  onClick={() => removePort(i)}
                  disabled={busy}
                  aria-label="Remove port mapping"
                  className="rounded p-1.5 text-ink-muted hover:text-bad"
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              </div>
            ))}
            {ports.length === 0 && <p className="text-xs text-ink-faint">No ports published.</p>}
          </div>
        </div>

        <div>
          <label className="mb-1 block text-xs font-medium text-ink-muted" htmlFor="cc-cmd">
            Command (optional)
          </label>
          <input
            id="cc-cmd"
            type="text"
            placeholder="nginx -g 'daemon off;'"
            value={command}
            disabled={busy}
            onChange={(e) => setCommand(e.target.value)}
            className={`${inputClass} font-mono`}
          />
        </div>

        <div>
          <label className="mb-1 block text-xs font-medium text-ink-muted" htmlFor="cc-mem">
            Memory limit (MiB, optional)
          </label>
          <input
            id="cc-mem"
            type="number"
            min={0}
            placeholder="512"
            value={memoryMb}
            disabled={busy}
            onChange={(e) => setMemoryMb(e.target.value)}
            className={inputClass}
          />
        </div>

        <div>
          <label className="mb-1 block text-xs font-medium text-ink-muted" htmlFor="cc-env">
            Environment variables (one per line, KEY=value)
          </label>
          <textarea
            id="cc-env"
            rows={3}
            value={envLines}
            disabled={busy}
            onChange={(e) => setEnvLines(e.target.value)}
            placeholder="NODE_ENV=production"
            className={`${inputClass} resize-none font-mono`}
          />
        </div>

        <div>
          <label className="mb-1 block text-xs font-medium text-ink-muted" htmlFor="cc-binds">
            Volume binds (one per line, hostPath:containerPath[:ro])
          </label>
          <textarea
            id="cc-binds"
            rows={2}
            value={bindLines}
            disabled={busy}
            onChange={(e) => setBindLines(e.target.value)}
            placeholder="/host/data:/data"
            className={`${inputClass} resize-none font-mono`}
          />
          <p className="mt-1 text-xs text-ink-faint">
            No path restrictions are applied — this app already has full Docker socket access.
          </p>
        </div>

        {error && <FormError>{error}</FormError>}
      </div>
    </Modal>
  );
}
