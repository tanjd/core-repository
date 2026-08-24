interface ContactRevealProps {
  name: string;
  email?: string;
  phone?: string;
  telegramUsername?: string;
  whatsappUsername?: string;
  contactNote?: string;
}

export function ContactReveal({
  name,
  email,
  phone,
  telegramUsername,
  whatsappUsername,
  contactNote,
}: ContactRevealProps) {
  const hasContact = [
    email,
    phone,
    telegramUsername,
    whatsappUsername,
    contactNote,
  ].some((v) => v && v.trim() !== "");

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
      {telegramUsername && telegramUsername.trim() !== "" && (
        <p className="text-sm">
          <span className="text-muted-foreground">Telegram: </span>
          <a
            href={`https://t.me/${telegramUsername.trim().replace(/^@/, "")}`}
            target="_blank"
            rel="noopener noreferrer"
            className="text-primary hover:underline"
          >
            {telegramUsername}
          </a>
        </p>
      )}
      {whatsappUsername && whatsappUsername.trim() !== "" && (
        <p className="text-sm">
          <span className="text-muted-foreground">WhatsApp: </span>
          {whatsappUsername}
        </p>
      )}
      {contactNote && contactNote.trim() !== "" && (
        <p className="text-sm whitespace-pre-wrap">
          <span className="text-muted-foreground">Note: </span>
          {contactNote}
        </p>
      )}
    </div>
  );
}
