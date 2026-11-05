import { useEffect, useState } from "react";
import { Link } from "react-router";
import { Star } from "lucide-react";
import { apiFetch } from "api/api";

interface AwardsSummary {
  gold: number[];
  silver: number[];
  bronze: number[];
  weeks_at_one: number;
  certification: string;
}

type ItemType = "Track" | "Album" | "Artist";

// Maloja's colours, kept local so the component does not depend on the theme.
const medalColors = {
  gold: "255,215,0",
  silver: "192,192,192",
  bronze: "205,127,50",
} as const;

const certColors: Record<string, string> = {
  gold: "255,215,0",
  platinum: "229,228,226",
  diamond: "103,161,253",
};

const rankLabel = { gold: "Best", silver: "Second best", bronze: "Third best" };

const chartPath = (type: ItemType) =>
  `/chart/top/${type.toLowerCase()}s`;

export default function Awards(props: { type: ItemType; id: number }) {
  const [summary, setSummary] = useState<AwardsSummary | null>(null);

  useEffect(() => {
    let cancelled = false;
    setSummary(null);
    apiFetch<AwardsSummary>(
      `/apis/web/v1/awards/${props.type.toLowerCase()}/${props.id}`,
    )
      .then((s) => {
        if (!cancelled) setSummary(s);
      })
      .catch(() => {
        if (!cancelled) setSummary(null);
      });
    return () => {
      cancelled = true;
    };
  }, [props.type, props.id]);

  if (!summary) return null;

  const medals = (["gold", "silver", "bronze"] as const).flatMap((kind) =>
    summary[kind].map((year) => ({ kind, year })),
  );
  medals.sort((a, b) => b.year - a.year);

  const cert = summary.certification;
  if (medals.length === 0 && summary.weeks_at_one === 0 && !cert) return null;

  return (
    <div className="flex flex-wrap items-center gap-2 mb-2">
      {cert && certColors[cert] && (
        <span
          title={`${cert.charAt(0).toUpperCase()}${cert.slice(1)} certified`}
          className="px-2 py-0.5 rounded-full text-xs font-semibold uppercase tracking-wide border"
          style={{
            color: `rgb(${certColors[cert]})`,
            borderColor: `rgb(${certColors[cert]})`,
            background: `rgba(${certColors[cert]},0.12)`,
          }}
        >
          {cert}
        </span>
      )}
      {summary.weeks_at_one > 0 && (
        <span
          title={`${summary.weeks_at_one} week${summary.weeks_at_one > 1 ? "s" : ""} at #1`}
          className="flex items-center gap-1 text-sm text-(--color-fg-secondary)"
        >
          <Star size={16} fill="currentColor" />
          {summary.weeks_at_one}
        </span>
      )}
      {medals.map(({ kind, year }) => (
        <Link
          key={`${kind}-${year}`}
          to={`${chartPath(props.type)}?year=${year}`}
          title={`${rankLabel[kind]} ${props.type.toLowerCase()} of ${year}`}
          className="px-2 py-0.5 rounded-full text-xs font-semibold border"
          style={{
            color: `rgb(${medalColors[kind]})`,
            borderColor: `rgb(${medalColors[kind]})`,
            background: `rgba(${medalColors[kind]},0.12)`,
          }}
        >
          {year}
        </Link>
      ))}
    </div>
  );
}
