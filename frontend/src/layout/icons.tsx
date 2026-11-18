import {
  Barcode,
  Boxes,
  Building2,
  ChartColumn,
  ChartLine,
  Circle,
  CircleUser,
  CloudUpload,
  Copy,
  Database,
  Dot,
  DollarSign,
  FileText,
  HeartPulse,
  House,
  type LucideIcon,
  Monitor,
  Settings,
  User,
  Users,
} from "lucide-react";

/**
 * Icon names usable in the `menus.icon` column. Only these icons are bundled;
 * add an entry here to support a new name. Unknown names render as a dot.
 */
const ICONS: Record<string, LucideIcon> = {
  home: House,
  database: Database,
  circle: Circle,
  users: Users,
  user: User,
  settings: Settings,
  heart: HeartPulse,
  chart: ChartLine,
  "bar-chart": ChartColumn,
  monitor: Monitor,
  boxes: Boxes,
  upload: CloudUpload,
  barcode: Barcode,
  dollar: DollarSign,
  file: FileText,
  "user-circle": CircleUser,
  copy: Copy,
  building: Building2,
};

export function iconFor(name: string): LucideIcon {
  return ICONS[name] ?? Dot;
}
