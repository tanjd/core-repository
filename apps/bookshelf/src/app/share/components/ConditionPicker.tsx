export type Condition = "good" | "fair" | "worn";

export const CONDITION_OPTIONS: {
  value: Condition;
  label: string;
  description: string;
}[] = [
  { value: "good", label: "Good", description: "Like new or minimal wear" },
  { value: "fair", label: "Fair", description: "Some wear, fully readable" },
  { value: "worn", label: "Worn", description: "Heavy wear but intact" },
];

export function ConditionPicker({
  value,
  onChange,
}: {
  value: Condition;
  onChange: (v: Condition) => void;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <label className="text-sm font-medium">Condition</label>
      <div className="flex gap-2">
        {CONDITION_OPTIONS.map((opt) => (
          <button
            key={opt.value}
            type="button"
            onClick={() => onChange(opt.value)}
            className={`flex-1 flex flex-col items-center gap-0.5 rounded-lg border px-3 py-2.5 text-sm transition-colors ${
              value === opt.value
                ? "border-primary bg-primary/5 text-primary font-medium"
                : "border-input text-muted-foreground hover:bg-accent hover:text-foreground"
            }`}
          >
            <span className="font-medium text-sm">{opt.label}</span>
            <span className="text-[11px] leading-tight text-center opacity-80">
              {opt.description}
            </span>
          </button>
        ))}
      </div>
    </div>
  );
}
