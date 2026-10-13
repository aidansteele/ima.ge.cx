// The unit for bytes is padded so that sizes line up in monospace columns.
const sizes = ['B ', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];

export function formatBytes(bytes: number, decimals = 0) {
  if (!(bytes >= 1)) return `${bytes || 0} ${sizes[0]}`;

  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), sizes.length - 1);

  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}
