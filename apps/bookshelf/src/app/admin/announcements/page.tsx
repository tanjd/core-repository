"use client";

import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Plus, Pencil, Trash2 } from "lucide-react";
import { api } from "@/lib/api";
import type { Announcement, AnnouncementType } from "@/lib/types";
import {
  announcementTypeLabel,
  announcementTypeVariant,
} from "@/lib/announcements";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { Pagination } from "@/components/ui/Pagination";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from "@/components/ui/dialog";

const PAGE_SIZE = 20;

export default function AdminAnnouncementsPage() {
  const [announcements, setAnnouncements] = useState<Announcement[]>([]);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);

  // Create/edit dialog — editing === null means create mode.
  const [editing, setEditing] = useState<Announcement | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [formTitle, setFormTitle] = useState("");
  const [formBody, setFormBody] = useState("");
  const [formType, setFormType] = useState<AnnouncementType>("info");
  const [formActive, setFormActive] = useState(true);
  const [submitting, setSubmitting] = useState(false);

  // Delete confirm dialog
  const [deleteTarget, setDeleteTarget] = useState<Announcement | null>(null);
  const [deleteSubmitting, setDeleteSubmitting] = useState(false);

  useEffect(() => {
    loadAnnouncements(1);
  }, []);

  async function loadAnnouncements(p: number) {
    setLoading(true);
    try {
      const data = await api.adminListAnnouncements({
        page: p,
        page_size: PAGE_SIZE,
      });
      setAnnouncements(data.items);
      setTotalPages(data.total_pages);
      setTotal(Number(data.total));
      setPage(p);
    } finally {
      setLoading(false);
    }
  }

  function openCreate() {
    setEditing(null);
    setFormTitle("");
    setFormBody("");
    setFormType("info");
    setFormActive(true);
    setFormOpen(true);
  }

  function openEdit(a: Announcement) {
    setEditing(a);
    setFormTitle(a.title);
    setFormBody(a.body);
    setFormType(a.type);
    setFormActive(a.active);
    setFormOpen(true);
  }

  async function handleSubmit() {
    setSubmitting(true);
    try {
      if (editing) {
        await api.adminUpdateAnnouncement(editing.id, {
          title: formTitle,
          body: formBody,
          type: formType,
          active: formActive,
        });
        toast.success("Announcement updated");
      } else {
        await api.adminCreateAnnouncement({
          title: formTitle,
          body: formBody,
          type: formType,
          active: formActive,
        });
        toast.success("Announcement created");
      }
      setFormOpen(false);
      loadAnnouncements(page);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Save failed");
    } finally {
      setSubmitting(false);
    }
  }

  async function toggleActive(a: Announcement) {
    try {
      await api.adminUpdateAnnouncement(a.id, { active: !a.active });
      setAnnouncements((prev) =>
        prev.map((x) => (x.id === a.id ? { ...x, active: !a.active } : x)),
      );
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Update failed");
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    setDeleteSubmitting(true);
    try {
      await api.adminDeleteAnnouncement(deleteTarget.id);
      toast.success("Announcement deleted");
      setDeleteTarget(null);
      loadAnnouncements(page);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Delete failed");
    } finally {
      setDeleteSubmitting(false);
    }
  }

  if (loading)
    return <p className="text-muted-foreground">Loading announcements…</p>;

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <p className="text-sm text-muted-foreground">
          {total} announcement{total !== 1 ? "s" : ""}
        </p>
        <Button size="sm" onClick={openCreate}>
          <Plus className="size-4" />
          New Announcement
        </Button>
      </div>

      <div className="rounded-md border overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/50">
              <th className="px-4 py-3 text-left font-medium">Title</th>
              <th className="px-4 py-3 text-left font-medium">Type</th>
              <th className="px-4 py-3 text-left font-medium">Active</th>
              <th className="px-4 py-3 text-left font-medium">Created</th>
              <th className="px-4 py-3 text-right font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {announcements.length === 0 ? (
              <tr>
                <td
                  colSpan={5}
                  className="px-4 py-6 text-center text-muted-foreground"
                >
                  No announcements yet.
                </td>
              </tr>
            ) : (
              announcements.map((a) => (
                <tr
                  key={a.id}
                  className={`border-b last:border-0 hover:bg-muted/30 ${!a.active ? "opacity-60" : ""}`}
                >
                  <td className="px-4 py-3 font-medium max-w-xs truncate">
                    {a.title}
                  </td>
                  <td className="px-4 py-3">
                    <Badge variant={announcementTypeVariant[a.type]}>
                      {announcementTypeLabel[a.type]}
                    </Badge>
                  </td>
                  <td className="px-4 py-3">
                    <Switch
                      checked={a.active}
                      onCheckedChange={() => toggleActive(a)}
                      aria-label={
                        a.active
                          ? "Deactivate announcement"
                          : "Activate announcement"
                      }
                    />
                  </td>
                  <td className="px-4 py-3 text-muted-foreground">
                    {new Date(a.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex gap-2 justify-end">
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => openEdit(a)}
                      >
                        <Pencil className="size-3" /> Edit
                      </Button>
                      <Button
                        size="sm"
                        variant="ghost"
                        className="text-destructive hover:text-destructive hover:bg-destructive/10"
                        onClick={() => setDeleteTarget(a)}
                      >
                        <Trash2 className="size-3" /> Delete
                      </Button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {totalPages > 1 && (
        <div className="mt-4">
          <Pagination
            page={page}
            totalPages={totalPages}
            onPageChange={loadAnnouncements}
          />
        </div>
      )}

      {/* Create/edit dialog */}
      <Dialog open={formOpen} onOpenChange={setFormOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {editing ? "Edit Announcement" : "New Announcement"}
            </DialogTitle>
          </DialogHeader>
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="announcement-title">Title</Label>
              <Input
                id="announcement-title"
                value={formTitle}
                onChange={(e) => setFormTitle(e.target.value)}
                placeholder="e.g. We're in beta!"
              />
            </div>

            <div className="flex flex-col gap-1.5">
              <Label htmlFor="announcement-body">Body</Label>
              <Textarea
                id="announcement-body"
                className="resize-none"
                value={formBody}
                onChange={(e) => setFormBody(e.target.value)}
                placeholder="e.g. Have feedback? PM me anytime."
              />
            </div>

            <div className="flex flex-col gap-1.5">
              <Label>Type</Label>
              <Select
                value={formType}
                onValueChange={(v) => setFormType(v as AnnouncementType)}
              >
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="info">Info</SelectItem>
                  <SelectItem value="new_feature">New feature</SelectItem>
                  <SelectItem value="known_issue">Known issue</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="flex items-center gap-2">
              <Switch
                id="announcement-active"
                checked={formActive}
                onCheckedChange={setFormActive}
              />
              <Label
                htmlFor="announcement-active"
                className="text-sm font-normal cursor-pointer"
              >
                Active
              </Label>
            </div>
          </div>
          <DialogFooter showCloseButton>
            <Button
              onClick={handleSubmit}
              disabled={submitting || !formTitle.trim() || !formBody.trim()}
            >
              {submitting ? "Saving…" : editing ? "Save changes" : "Create"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete confirm dialog */}
      <Dialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete this announcement?</DialogTitle>
            <DialogDescription>
              {deleteTarget
                ? `This permanently removes "${deleteTarget.title}". This can't be undone.`
                : "This can't be undone."}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter showCloseButton>
            <Button
              variant="destructive"
              onClick={handleDelete}
              disabled={deleteSubmitting}
            >
              {deleteSubmitting ? "Deleting…" : "Delete announcement"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
