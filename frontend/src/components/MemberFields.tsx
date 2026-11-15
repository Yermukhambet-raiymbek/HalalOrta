import { useId } from "react";
import { RELATIONSHIPS } from "../api";

type Props = {
  name: string;
  onNameChange: (name: string) => void;
  relationship: string;
  onRelationshipChange: (relationship: string) => void;
  disabled: boolean;
};

/** Name and relationship inputs shared by the create and join forms. */
export default function MemberFields({ name, onNameChange, relationship, onRelationshipChange, disabled }: Props) {
  const id = useId();
  return (
    <>
      <div className="field">
        <label htmlFor={`${id}-name`}>Your name</label>
        <input
          id={`${id}-name`}
          value={name}
          onChange={(e) => onNameChange(e.target.value)}
          maxLength={40}
          required
          autoComplete="given-name"
          disabled={disabled}
        />
      </div>
      <div className="field">
        <label htmlFor={`${id}-relationship`}>Your relationship to the baby</label>
        <select
          id={`${id}-relationship`}
          value={relationship}
          onChange={(e) => onRelationshipChange(e.target.value)}
          required
          disabled={disabled}
        >
          <option value="" disabled>
            Choose…
          </option>
          {RELATIONSHIPS.map((r) => (
            <option key={r} value={r}>
              {r}
            </option>
          ))}
        </select>
      </div>
    </>
  );
}
