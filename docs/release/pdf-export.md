# Frozen Protocol and Run PDF Export

The Run page exports the verified frozen AsterDoc and its linked Run records as one PDF. The exporter rejects a frozen source whose content no longer matches its pinned source hash.

The PDF includes the document revision, source hash, Run ID and Run revision; the Core freeze-audit timestamp when present; the frozen plan and static module preview; and Run records in workspace order. Grouped steps are expanded to their child tasks. Planned parameters, recorded parameter differences, readouts, associated event details and unassociated records follow the fields shown by the Run workspace. Older Runs without a creation audit are labeled as having no recorded freeze time.

Run record text is captured before the asynchronous export begins, so the worker does not read mutable UI state. The exporter wraps and paginates long content, selects a CJK font when the content requires it, and fixes PDF creation/modification timestamps for repeatable output from the same inputs. The UI writes to a temporary file beside the destination, syncs it, then renames it into place.

This is a local export. It does not modify the frozen document or Run and does not upload data.