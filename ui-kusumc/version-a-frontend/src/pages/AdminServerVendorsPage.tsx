import { VendorsSection } from '../features/admin/vendors/VendorsSection';

export function AdminServerVendorsPage() {
  return (
    <div className="space-y-6">
      <VendorsSection initialCollection="server" collections={['server']} />
    </div>
  );
}
