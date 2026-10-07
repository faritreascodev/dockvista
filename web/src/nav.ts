import { Boxes, Gauge, HardDrive, Hexagon, Image as ImageIcon, Layers, Network, Trash2 } from "lucide-react";
import type { LucideIcon } from "lucide-react";

export interface NavItem {
  label: string;
  path: string;
  icon: LucideIcon;
}

export const NAV_SECTIONS: { title: string; items: NavItem[] }[] = [
  { title: "Monitor", items: [{ label: "Overview", path: "/overview", icon: Gauge }] },
  {
    title: "Workloads",
    items: [
      { label: "Containers", path: "/containers", icon: Boxes },
      { label: "Compose", path: "/compose", icon: Layers },
      { label: "Swarm", path: "/swarm", icon: Hexagon },
    ],
  },
  {
    title: "Resources",
    items: [
      { label: "Images", path: "/images", icon: ImageIcon },
      { label: "Volumes", path: "/volumes", icon: HardDrive },
      { label: "Networks", path: "/networks", icon: Network },
      { label: "Storage", path: "/storage", icon: Trash2 },
    ],
  },
];
