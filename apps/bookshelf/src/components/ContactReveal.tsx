interface ContactRevealProps {
  name: string;
  email?: string;
  phone?: string;
}

export function ContactReveal({ name, email, phone }: ContactRevealProps) {
  const hasContact =
    (email && email.trim() !== "") || (phone && phone.trim() !== "");

  if (!hasContact) {
    return (
      <div className="rounded-md border border-dashed p-4 text-sm text-muted-foreground">
        Contact info not available
      </div>
    );
  }

  return (
    <div className="rounded-md border bg-muted/50 p-4 flex flex-col gap-1">
      <p className="font-medium text-sm">{name}</p>
      {email && email.trim() !== "" && (
        <p className="text-sm">
          <span className="text-muted-foreground">Email: </span>
          <a href={`mailto:${email}`} className="text-primary hover:underline">
            {email}
          </a>
        </p>
      )}
      {phone && phone.trim() !== "" && (
        <p className="text-sm">
          <span className="text-muted-foreground">Phone: </span>
          <a href={`tel:${phone}`} className="text-primary hover:underline">
            {phone}
          </a>
        </p>
      )}
    </div>
  );
}
