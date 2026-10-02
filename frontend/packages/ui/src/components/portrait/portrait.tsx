// The image server only serves power-of-two sizes.
function imageSize(size: number) {
  let s = 32;
  while (s < size && s < 1024) s *= 2;
  return s;
}

export function Portrait({ id, size, className }: { id: number; size: number; className?: string }) {
  return (
    <img
      src={`https://images.evetech.net/characters/${id}/portrait?size=${imageSize(size)}`}
      alt=""
      width={size}
      height={size}
      className={className}
    />
  );
}
