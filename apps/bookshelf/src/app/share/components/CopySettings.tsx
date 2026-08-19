import { Switch } from "@/components/ui/switch";

export function CopySettings({
  autoApprove,
  returnDateRequired,
  hideOwner,
  onAutoApproveChange,
  onReturnDateRequiredChange,
  onHideOwnerChange,
}: {
  autoApprove: boolean;
  returnDateRequired: boolean;
  hideOwner: boolean;
  onAutoApproveChange: (v: boolean) => void;
  onReturnDateRequiredChange: (v: boolean) => void;
  onHideOwnerChange: (v: boolean) => void;
}) {
  return (
    <div className="flex flex-col gap-4">
      {[
        {
          id: "auto-approve",
          label: "Auto-approve requests",
          description:
            "Loan requests are accepted automatically without your review",
          checked: autoApprove,
          onChange: onAutoApproveChange,
        },
        {
          id: "return-date",
          label: "Require return date",
          description:
            "Borrowers must specify when they plan to return the book",
          checked: returnDateRequired,
          onChange: onReturnDateRequiredChange,
        },
        {
          id: "hide-owner",
          label: "Stay anonymous",
          description: "Your name is hidden from borrowers in the catalog",
          checked: hideOwner,
          onChange: onHideOwnerChange,
        },
      ].map(({ id, label, description, checked, onChange }) => (
        <div key={id} className="flex items-center justify-between gap-4">
          <div>
            <p className="text-sm font-medium">{label}</p>
            <p className="text-xs text-muted-foreground mt-0.5">
              {description}
            </p>
          </div>
          <Switch id={id} checked={checked} onCheckedChange={onChange} />
        </div>
      ))}
    </div>
  );
}
